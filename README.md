# turkey-ops
disclaimer: super early pre requirement signoff version a.k.a just commit history will be sqashed, mess codes will be cleaned up/refactored, branching policy/gitops/deployment channel will be established etc.

## folders
### .github/workflows
github action pipelines, since i'm fastest with it, can be moved to jenkins later
contains:
- dialog-docker.yml
  - pulls code from dialog repo
  - docker build and docker push to DOCKER_HUB_USR }}/dialog:latest
- hubs-rawhubs-s3.yml
  - pulls code from hubs repo
  - npm build hubs and admin
  - repackage files as per HC's asset-s3-bucket, and push the files to s3://turkeycfg/rawhubs
- ret-bio-docker.yml
  - pulls code from ret repo
  - add a simple code change (to avoid impact dev, will be replaced with a PR)
  - bio-pkg-export-container into a docker images
  - repackage the docker image with self-signed certs and push to OCKER_HUB_USR }}/ret:latest
### cf-templates
nested cloudformation templates used as part of turkey's orchestration process
### orchestrator
a golang service does the following:
(atm)
- handles a POST at /TurkeyDeployAWS
- reads a json input for aws creds and config overrides
- deploys turkey on aws, from infra to code to configs
- report status
(eventually)
should act as a broker between turkey's frontend (console / mgmt ui) and infra backend, by serving all infra/ops related endpoints such as:
- deployments and deletions
- queries (status / logs / usages / billings )
- updates (code / infra / config )


