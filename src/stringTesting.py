import yaml, boto3, botocore, json, zipfile
from os import path
from kubernetes import client, config
from string import Template

s3 = boto3.resource('s3')
code_pipeline = boto3.client('codepipeline')
ssm = boto3.client('ssm')

def main():

    # Build config file from template and secrets in SSM
    CA = "THIS IS A CA"
    CLIENT_CERT = "THIS IS A CLIENT CERT"
    CLIENT_KEY = "THIS IS A CLIENT KEY"
    ENDPOINT= "THIS IS AN ENDPOINT"
    filein = open('/tmp/config')
    src = Template(filein.read())

    d={'$CA': CA, '$CLIENT_CERT': CLIENT_CERT, '$CLIENT_KEY': CLIENT_KEY, '$ENDPOINT': ENDPOINT}

    result= src.substiute(d)
    config.load_kube_config('/tmp/config')


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

main()