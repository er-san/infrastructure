setup:

```
cd pulumi/state

make login

#passphrase is empty
pulumi up -y

pulumi stack output

cd ...
```

edit the makefile here to point to the previous pulumi stack output's bucket name for s3

in the main folder:

```
make login
```

creating new stacks after logging into the global s3 state:
note the hyphen in the name of the project, infrastructure being prefix for identifier and namespacing

```
from the KMS output copy in the key id:
pulumi new aws-typescript --force --name 'infrastructure-k8s' --secrets-provider awskms://8008ba8f-60bd-446f-ba4e-addb16425a29 -s dev

if youre using the existing template code here in this repo, reset the files:
git checkout ./.
```
