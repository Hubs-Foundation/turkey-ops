
[ -z "$s3bkt" ] && s3bkt="turkeycfg"
[ -z "$env" ] && env="dev"

#aws s3 sync --exclude "*" --include "*.yaml" ./cfs s3://$s3bkt/$env/cfs
#aws s3 sync --exclude "*" --include "*.yam" ./yams s3://$s3bkt/$env/yams
aws s3 sync . s3://$s3bkt/$env --exclude "*" --include "*.yaml" --include "*.yam"