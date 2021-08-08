env="dev"
! [ -z "$1" ] && env=$1
s3path="s3://turkeycfg/cf/$env"
echo "pushing $s3path"
aws s3 sync --exclude "*" --include "*.yaml" . s3://turkeycfg/cf/$env
