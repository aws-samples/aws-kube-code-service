import yaml, boto3, botocore, json, zipfile
from os import path
from kubernetes import client, config

s3 = boto3.resource('s3')
code_pipeline = boto3.client('codepipeline')
ssm = boto3.client('ssm')

def lambda_handler(event, context):

    cplJobId = event['CodePipeline.job']['id']
    cplKey = event['CodePipeline.job']['data']['inputArtifacts'][0]['location']['s3Location']['objectKey']
    cplBucket = event['CodePipeline.job']['data']['inputArtifacts'][0]['location']['s3Location']['bucketName']

    s3.meta.client.download_file(cplBucket,cplKey,'/tmp/build.zip')

    zip_ref = zipfile.ZipFile('/tmp/build.zip', 'r')
    zip_ref.extractall('/tmp/')
    zip_ref.close()

    with open('/tmp/build.json') as json_data:
        d = json.load(json_data)

    s3.meta.client.download_file(d["template-bucket"], 'web-server-deployment.yml', '/tmp/web-server-deployment.yml')
    s3.meta.client.download_file(d["template-bucket"], 'config', '/tmp/config')
    print(d["repository-uri"], d["tag"], d["deployment-name"])

    inplace_change("/tmp/web-server-deployment.yml", "$REPOSITORY_URI", d["repository-uri"])
    inplace_change("/tmp/web-server-deployment.yml", "$TAG", d["tag"])

    # Build config file from template and secrets in SSM
    CA = ssm.get_parameter(Name='CA', WithDecryption=True)["Parameter"]["Value"]
    CLIENT_CERT = ssm.get_parameter(Name='ClientCert', WithDecryption=True)["Parameter"]["Value"]
    CLIENT_KEY = ssm.get_parameter(Name='ClientKey', WithDecryption=True)["Parameter"]["Value"]

    inplace_change("/tmp/config", "$ENDPOINT", d["cluster-endpoint"])
    inplace_change("/tmp/config", "$CA", CA)
    inplace_change("/tmp/config", "$CLIENT_CERT", CLIENT_CERT)
    inplace_change("/tmp/config", "$CLIENT_KEY", CLIENT_KEY)

    config.load_kube_config('/tmp/config')
    try:
        with open(path.join(path.dirname(__file__), "/tmp/web-server-deployment.yml")) as f:
            dep = yaml.load(f)
            k8s_beta = client.ExtensionsV1beta1Api()
            resp = k8s_beta.patch_namespaced_deployment(name=d["deployment-name"],
                                                        body=dep, namespace="default")
            print("Deployment created. status='%s'" % str(resp.status))

        code_pipeline.put_job_success_result(jobId=cplJobId)
        return 'Success'
    except Exception as e:
        code_pipeline.put_job_failure_result(jobId=cplJobId, failureDetails={'message': 'Job Failed', 'type': 'JobFailed'})
        print(e)
        raise e

def inplace_change(filename, old_string, new_string):
    with open(filename) as f:
        s = f.read()
        if old_string not in s:
            # print '"{old_string}" not found in {filename}.'.format(**locals())
            return

    with open(filename, 'w') as f:
        # print 'Changing "{old_string}" to "{new_string}" in {filename}'.format(**locals())
        s = s.replace(old_string, new_string)
        f.write(s)
