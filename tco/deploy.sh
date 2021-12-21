env="dev"
! [ -z "$1" ] && env=$1
s3bkt=turkeycfg

aws s3 sync --exclude "*" --include "*.yaml" ./cf s3://$s3bkt/$env/cf
aws s3 sync --exclude "*" --include "*.yaml" ./k8s s3://$s3bkt/$env/k8s
aws s3 cp hc.yam s3://$s3bkt/$env/hc.yam
