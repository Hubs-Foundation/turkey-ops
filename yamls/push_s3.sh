
[ -z "$s3bkt" ] && s3bkt="turkeycfg"
[ -z "$env" ] && env="dev"

s3SyncTo="s3://$s3bkt/$env"
echo "[info] pushing to: $s3SyncTo"

aws s3 sync . $s3SyncTo --exclude "*" --include "*.yaml" --include "*.yam"