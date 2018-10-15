# Continuous Deployment Sample using AWS CodePipeline

## Building

You will need to setup Go environment to build this function. Refer to this [Getting Started
Guide](https://golang.org/install) and [installing
dep](https://golang.github.io/dep/docs/installation.html) for detailed instructions.

To build this function, copy this directory to your `$GOPATH/src` directory and then execute the
`make` command.

```
$ make
```

This creates a zip package of the function which can be deployed to AWS Lambda. 

## Configuration

You would need to give the Lambda execution role permissions in Amazon EKS cluster. Refer to
this [User Guide](https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html) for detailed
instructions.

1. Edit the `aws-auth` ConfigMap of your cluster.
```
$ kubectl -n kube-system edit configmap/aws-auth
```
2. Add your Lambda execution role to the config
```
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
```

For your Lambda execution role, you will need permissions to describe EKS cluster. Add the following
statement to the IAM role.

```
{
    "Effect": "Allow",
    "Action": [
        "eks:DescribeCluster"
    ],
    "Resource": "*"
}
```

You may want to be more restrictive by specifying only the arn of your EKS cluster for resource
field.

Once these are configured, you can test your function. Good luck!

## Deployment

This reference architecture can be deployed using the AWS CloudFormation template below.

[<img
src="https://s3.amazonaws.com/cloudformation-examples/cloudformation-launch-stack.png">](https://console.aws.amazon.com/cloudformation/home?region=us-west-2#/stacks/new?stackName=Codesuite-Demo&templateURL=https://s3-us-west-2.amazonaws.com/aws-eks-codesuite/aws-refarch-codesuite-kubernetes.yaml)


