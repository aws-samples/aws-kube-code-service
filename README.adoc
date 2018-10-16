:icons:
:linkcss:
:imagesdir: ./images

= CodeSuite - Continuous Deployment Reference Architecture for Kubernetes

The CodeSuite Continuous Deployment reference architecture demonstrates how to achieve continuous
deployment of an application to a Kubernetes cluster using AWS CodePipeline, AWS CodeCommit, AWS CodeBuild and AWS Lambda.

Launching this AWS CloudFormation stack provisions a continuous deployment process that uses AWS CodePipeline
to monitor an AWS CodeCommit repository for new commits, AWS CodeBuild to create a new Docker container image and to push
it into Amazon ECR. Finally an AWS Lambda function with the Kubernetes Python SDK updates a Kubernetes deployment in a live cluster.

When you deploy the cloudformation stack there will be four parameters that are specific to your Kubernetes cluster. You will need the API endpoint (enter only the subdomain and omit 'api'), Certificate Authority Data, Client Certificate Data and Client Key Data.
The last of these three are sensitive, the cloudformation parameter is marked with the "NoEcho" property set to true so that the contents are not exposed through cloudformation. In addition those strings are encrypted with the account default
KMS key and stored in parameter store. The Lambda function that authenticates to your Kubernetes API endpoint is assigned an IAM role that has permission to access those keys. The Lambda function builds a config file in the tmpfs directory of the Lambda which is in memory
so that when the Lambda function terminates the secrets are gone.

image::architecture.png[Architecture]

=== Pre-Requisites

A functioning Kubernetes cluster and config file to authenticate to the cluster, by default this is located at `~/.kube/config`

Clone this repository

    git clone https://github.com/aws-samples/aws-kube-codesuite

This creates a directory named `aws-kube-codesuite` in your current directory, which contains the code we need for this tutorial. Change to this directory.

=== Application - initial deployment and service Provisioning

    kubectl apply -f ./kube-manifests/deploy-first.yml

Find the service endpoint to view the application:

    kubectl get svc codesuite-demo -o wide

If you copy and paste the External IP from the codesuite-demo service into a browser you should see the nginx homepage.

=== Deploy the CloudFormation stack

Note, deploy this stack in the same region as your k8s cluster. Your cluster nodes will require access via an IAM profile to download images from ECR. If you deployed this cluster through KOPS this will be already take care of for you.

|===

|Region | Launch Template for Kubernetes | Launch Template for Amazon EKS
| *N. Virginia* (us-east-1)
a| image::./deploy-to-aws.png[link=https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3.amazonaws.com/codesuite-demo-public/aws-refarch-codesuite-kubernetes.yaml]
a| image::./deploy-to-aws.png[link=https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3-us-west-2.amazonaws.com/aws-eks-codesuite/aws-refarch-codesuite-kubernetes.yaml]

| *Ohio* (us-east-2)
a| image::./deploy-to-aws.png[link=https://console.aws.amazon.com/cloudformation/home?region=us-east-2#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3.amazonaws.com/codesuite-demo-public/aws-refarch-codesuite-kubernetes.yaml]
a|

| *Oregon* (us-west-2)
a| image::./deploy-to-aws.png[link=https://console.aws.amazon.com/cloudformation/home?region=us-west-2#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3.amazonaws.com/codesuite-demo-public/aws-refarch-codesuite-kubernetes.yaml]
a| image::./deploy-to-aws.png[link=https://console.aws.amazon.com/cloudformation/home?region=us-west-2#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3-us-west-2.amazonaws.com/aws-eks-codesuite/aws-refarch-codesuite-kubernetes.yaml]

| *Ireland* (eu-west-1)
a| image::./deploy-to-aws.png[link=https://console.aws.amazon.com/cloudformation/home?region=eu-west-1#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3.amazonaws.com/codesuite-demo-public/aws-refarch-codesuite-kubernetes.yaml]
a| image::./deploy-to-aws.png[link=https://console.aws.amazon.com/cloudformation/home?region=eu-west-1#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3-us-west-2.amazonaws.com/aws-eks-codesuite/aws-refarch-codesuite-kubernetes.yaml]

| *Frankfurt* (eu-central-1)
a| image::./deploy-to-aws.png[link=https://console.aws.amazon.com/cloudformation/home?region=eu-central-1#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3.amazonaws.com/codesuite-demo-public/aws-refarch-codesuite-kubernetes.yaml]
a|

| *Singapore* (ap-southeast-1)
a| image::./deploy-to-aws.png[link=https://console.aws.amazon.com/cloudformation/home?region=ap-southeast-1#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3.amazonaws.com/codesuite-demo-public/aws-refarch-codesuite-kubernetes.yaml]
a|

| *Sydney* (ap-southeast-2)
a| image::./deploy-to-aws.png[link=https://console.aws.amazon.com/cloudformation/home?region=ap-southeast-2#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3.amazonaws.com/codesuite-demo-public/aws-refarch-codesuite-kubernetes.yaml]
a|

| *Tokyo* (ap-northeast-1)
a| image::./deploy-to-aws.png[link=https://console.aws.amazon.com/cloudformation/home?region=ap-northeast-1#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3.amazonaws.com/codesuite-demo-public/aws-refarch-codesuite-kubernetes.yaml]
a|

|===

If you are deploying this architecture to an Amazon EKS cluster, you would need to give the Lambda
execution role permissions in Amazon EKS cluster. You can get the ARN of your Lambda execution role
from the Outputs tab in the CloudFormation template. Refer to this 
link:https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html[User Guide] for detailed
instructions.

1. Edit the `aws-auth` ConfigMap of your cluster.

    kubectl -n kube-system edit configmap/aws-auth

2. Add your Lambda execution role to the config

    # Please edit the object below. Lines beginning with a '#' will be ignored,
    # and an empty file will abort the edit. If an error occurs while saving this file will be
    # reopened with the relevant failures.
    #
    apiVersion: v1
    data:
      mapRoles: |
        - rolearn: arn:aws:iam::<AWS Account ID>:role/devel-worker-nodes-NodeInstanceRole-74RF4UBDUKL6
          username: system:node:{{EC2PrivateDNSName}}
          groups:
            - system:bootstrappers
            - system:nodes
        - rolearn: arn:aws:iam::<AWS Account ID>:role/<your lambda execution role>
          username: admin
          groups:
            - system:masters

=== Test CI/CD platform

Install credential helper

    git config --global credential.helper '!aws codecommit credential-helper $@'
    git config --global credential.UseHttpPath true

Clone CodeCommit Repository (url will be in CloudFormation Output), change directories up one level `cd ..` so that both repositories are at the same directory structure.
Check the outputs page for your CloudFormation stack:

    git clone <name_of_your_codecommit_repository>

This creates a directory named `codesuite-demo` in your current directory.

Copy contents from aws-kube-codesuite/sample-app to this repository folder.

    cp aws-kube-codesuite/sample-app/* codesuite-demo/

Make a change to the `codesuite-demo/hello.py` file and then change into that directory `cd codesuite-demo`

Add, commit and push:

    git add . && git commit -m "test CodeSuite" && git push origin master

To view the pipeline in the AWS console go to the outputs tab for the pipeline cloudformation template and click on the Pipeline URL link:

image::pipeline-url.png[pipeline-url]

You can then see the pipeline move through the various stages:

image::pipeline.png[pipeline]

Once the final Lambda stage is complete you should be able to see the new deployment exposed through the same service load balancer.

    kubectl get svc codesuite-demo -o wide

Now if you copy and paste the External IP from the codesuite-demo service into a browser you should see the flask page reflecting the changes you applied.

=== Cleaning up the example resources

To remove all resources created by this example do the following:

1. Delete the main CloudFormation stack which deletes the substacks and resources.
2. Manually delete resources which may contain files:
* S3 bucket: ArtifactBucket
* S3 bucket: LambdaCopy bucket
* ECR repository: Repository
3. Delete the Kubernetes deployment and service

== CloudFormation template resources

The following section explains all of the resources created the CloudFormation template provided with this example.

link:/templates/lambda-copy.yaml[lambda-copy]

This creates a Lambda function that copies the Lambda code from the central account into the user account.

link:/templates/ssm-inject.yaml[ssm-inject]

Deploys a custom resource via Lambda which creates secure string key value pairs for all of the secrets required to authenticate to the Kubernetes cluster.

link:/templates/deployment-pipeline.yaml[deployment-pipeline]

Resources that compose the deployment pipeline include the CodeBuild project, the CodePipeline pipeline, an S3 bucket for deployment artifacts, and ECR repository for the container images and all necessary IAM roles used by those services.

== License Summary

This sample code is made available under a modified MIT license. See the LICENSE file.
