import yaml, boto3, botocore, json, zipfile
from os import path
from kubernetes import client, config

s3 = boto3.resource('s3')

s3.meta.client.download_file("olari-codestar-blog", 'build.zip', '/Users/olari/OneDrive/sandbox/scratch2/build.zip')

zip_ref = zipfile.ZipFile('/Users/olari/OneDrive/sandbox/scratch2/build.zip', 'r')
zip_ref.extractall('/Users/olari/OneDrive/sandbox/scratch2/')
zip_ref.close()


