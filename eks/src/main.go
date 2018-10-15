package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	eksauth "github.com/chankh/eksutil/pkg/auth"
	"github.com/pkg/errors"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codepipeline"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	log "github.com/sirupsen/logrus"
)

const tmp = "/tmp/"
const buildFile = tmp + "build.zip"

var sess = session.Must(session.NewSession())
var s3Downloader = s3manager.NewDownloader(sess)
var cp = codepipeline.New(sess)

func main() {
	if os.Getenv("ENV") == "DEBUG" {
		log.SetLevel(log.DebugLevel)
	}

	lambda.Start(handler)
}

func handler(context context.Context, req events.CodePipelineEvent) {

	cplJobID := req.CodePipelineJob.ID
	cplKey := req.CodePipelineJob.Data.InputArtifacts[0].Location.S3Location.ObjectKey
	cplBucket := req.CodePipelineJob.Data.InputArtifacts[0].Location.S3Location.BucketName
	log.WithFields(log.Fields{
		"jobID":  cplJobID,
		"bucket": cplBucket,
		"key":    cplKey,
	}).Info("Deployment started")

	if err := s3Download(cplJobID, cplBucket, cplKey, buildFile); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"jobID":    cplJobID,
			"bucket":   cplBucket,
			"key":      cplKey,
			"filename": buildFile,
		}).Error("failed to download file")
		failJob(cplJobID, "failed to download file", err)
		return
	}

	// Extract contents of zip to our tmp location
	if err := extractZip(buildFile, tmp); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"jobID":    cplJobID,
			"filename": buildFile,
		}).Error("failed extracting zip file")
		failJob(cplJobID, "failed extracting zip file", err)
		return
	}

	d, err := loadBuildData(tmp + "build.json")
	if err != nil {
		failJob(cplJobID, "failed to parse build.json", err)
		return
	}

	deployFile := tmp + "web-server-deployment.yml"

	if err = s3Download(cplJobID, d.TemplateBucket, "web-server-deployment.yml", deployFile); err != nil {
		failJob(cplJobID, "unable to get deployment file", err)
		return
	}

	log.WithFields(log.Fields{
		"jobID":          cplJobID,
		"deploymentName": d.DeploymentName,
		"eksCluster":     d.ClusterName,
		"repositoryURI":  d.RepositoryURI,
		"tag":            d.Tag,
	}).Info("Deploying to EKS")

	b, err := ioutil.ReadFile(deployFile)
	if err != nil {
		failJob(cplJobID, "unable to load deployment file", err)
		return
	}

	deployYAML := string(b)
	deployYAML = inplaceChange(deployYAML, "$REPOSITORY_URI", d.RepositoryURI)
	deployYAML = inplaceChange(deployYAML, "$TAG", d.Tag)

	var dep appsv1.Deployment
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(deployYAML), 100)
	err = decoder.Decode(&dep)
	if err != nil {
		failJob(cplJobID, "error parsing deployment file", err)
		return
	}

	cfg := &eksauth.ClusterConfig{
		ClusterName: d.ClusterName,
	}

	clientset, err := eksauth.NewAuthClient(cfg)
	if err != nil {
		failJob(cplJobID, "unable to authenticate with EKS", err)
		return
	}

	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)
	_, err = deploymentsClient.Update(&dep)
	if err != nil {
		failJob(cplJobID, "failed to update deployment", err)
		return
	}

	cp.PutJobSuccessResult(&codepipeline.PutJobSuccessResultInput{
		JobId: &cplJobID,
	})
}

func extractZip(zipFile, destination string) error {
	// Open a zip archive for reading.
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer r.Close()

	// Iterate through the files in the archive,
	// extracting them to destination.
	for _, f := range r.File {
		if err := unzipFile(f, destination); err != nil {
			return err
		}
	}

	return nil
}

func unzipFile(zf *zip.File, destination string) error {
	err := sanitizeExtractPath(zf.Name, destination)
	if err != nil {
		return err
	}

	if strings.HasSuffix(zf.Name, "/") {
		return mkdir(filepath.Join(destination, zf.Name))
	}

	rc, err := zf.Open()
	if err != nil {
		return errors.Errorf("%s: open compressed file: %v", zf.Name, err)
	}
	defer rc.Close()

	return writeNewFile(filepath.Join(destination, zf.Name), rc, zf.FileInfo().Mode())
}

func writeNewFile(path string, in io.Reader, fm os.FileMode) error {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return errors.Errorf("%s: making directory for file: %v", path, err)
	}

	out, err := os.Create(path)
	if err != nil {
		return errors.Errorf("%s: creating new file: %v", path, err)
	}
	defer out.Close()

	err = out.Chmod(fm)
	if err != nil && runtime.GOOS != "windows" {
		return errors.Errorf("%s: changing file mode: %v", path, err)
	}

	_, err = io.Copy(out, in)
	if err != nil {
		return errors.Errorf("%s: writing file: %v", path, err)
	}
	return nil
}

func mkdir(dirPath string) error {
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return errors.Errorf("%s: making directory: %v", dirPath, err)
	}
	return nil
}

func sanitizeExtractPath(filePath, destination string) error {
	// to avoid zip slip (writing outside of the destination), we resolve
	// the target path, and make sure it's nested in the intended
	// destination, or bail otherwise.
	destpath := filepath.Join(destination, filePath)
	if !strings.HasPrefix(destpath, filepath.Clean(destination)) {
		return errors.Errorf("%s: illegal file path", filePath)
	}
	return nil
}

func loadBuildData(filename string) (*BuildData, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %q", filename)
	}

	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed reading file %q", filename)
	}

	var data BuildData
	json.Unmarshal(byteValue, &data)

	return &data, nil
}

func s3Download(jobID, bucket, key, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	// Write the contents of S3 Object to the file
	_, err = s3Downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}

	return nil
}

func inplaceChange(input, oldString, newString string) string {
	return strings.Replace(input, oldString, newString, -1)
}

func failJob(jobID, message string, err error) {
	failType := "JobFailed"
	log.WithError(err).Fatal(message)
	cp.PutJobFailureResult(&codepipeline.PutJobFailureResultInput{
		JobId: &jobID,
		FailureDetails: &codepipeline.FailureDetails{
			Message: &message,
			Type:    &failType,
		},
	})
}

type BuildData struct {
	Tag            string `json:"tag"`
	RepositoryURI  string `json:"repository-uri"`
	TemplateBucket string `json:"template-bucket"`
	DeploymentName string `json:"deployment-name"`
	ClusterName    string `json:"cluster-name"`
}
