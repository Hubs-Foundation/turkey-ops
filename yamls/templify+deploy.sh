! [ -z "$s3bkt" ] && s3bkt="turkeycfg"
! [ -z "$env" ] && env="dev"
! [ -z "$target" ] && target="aws"


### templify
mkdir templified

#templify hubs-cloud-instance-scoped ns_hc-dev0.yaml
yam="templified/ns_hc.yam"
cp k8s/ns_hc-dev0.yaml $yam
sed -i 's/ret_dev0/{{.DBname}}/g' $yam
sed -i 's/dev0/{{.Subdomain}}/g' $yam
sed -i 's/TurkeyId: someString/TurkeyId: {{.TurkeyId}}/g' $yam
sed -i 's/myhubs.net/{{.Domain}}/g' $yam
sed -i 's/gtan@mozilla.com/{{.UserEmail}}/g' $yam
sed -i 's#  PERMS_KEY: ----.*#  PERMS_KEY: {{.PermsKey}}#g' $yam
sed -i 's#".*3RY0qLmdthY6Q0RZ4oyNQSL035BmYLNdleX1qVpG1zfQeLWf.*#"{{.JWK}}"#g' $yam

#templify turkey-cluster-scoped cluster_*.yaml(s)
yam="templified/cluster_00_deps.yam"
cp k8s/cluster_00_deps.yaml $yam

yam="templified/cluster_01_ingress.yam"
cp k8s/cluster_01_ingress.yaml $yam
sed -i 's/myhubs.net/{{.Domain}}/g' $yam

yam="templified/cluster_02_tools.yam"
cp k8s/cluster_02_tools.yaml $yam
sed -i 's/myhubs.net/{{.Domain}}/g' $yam

yam="templified/cluster_03_turkey-services.yam"
cp k8s/cluster_03_turkey-services.yaml $yam
sed -i 's/myhubs.net/{{.Domain}}/g' $yam
sed -i 's#  OAUTH_CLIENT_ID_FXA: secret#  OAUTH_CLIENT_ID_FXA: {{.OAUTH_CLIENT_ID_FXA}}#g' $yam
sed -i 's#  OAUTH_CLIENT_SECRET_FXA: secret#  OAUTH_CLIENT_SECRET_FXA: {{.OAUTH_CLIENT_SECRET_FXA}}#g' $yam
sed -i 's#  COOKIE_SECRET: secret#  COOKIE_SECRET: {{.COOKIE_SECRET}}#g' $yam
sed -i 's#  DB_CONN: secret#  DB_CONN: {{.DB_CONN}}#g' $yam
sed -i 's#  AWS_KEY: secret#  AWS_KEY: {{.AWS_KEY}}#g' $yam
sed -i 's#  AWS_SECRET: secret#  AWS_SECRET: {{.AWS_SECRET}}#g' $yam
sed -i 's#  AWS_REGION: secret#  AWS_REGION: {{.AWS_REGION}}#g' $yam
sed -i 's#  PERMS_KEY: secret#  PERMS_KEY: {{.PERMS_KEY}}#g' $yam
sed -i 's#  DB_PASS: secret#  DB_PASS: {{.DB_PASS}}#g' $yam
sed -i 's#  DB_HOST: secret#  DB_HOST: {{.DB_HOST}}#g' $yam

yam="templified/cluster_04_turkey-stream.yam"
cp k8s/cluster_04_turkey-stream.yaml $yam
sed -i 's#  PERMS_KEY: secret#  DB_HOST: {{.PERMS_KEY}}#g' $yam
sed -i 's#  PSQL: secret#  DB_HOST: {{.PSQL}}#g' $yam

#push to s3
aws s3 sync --exclude "*" --include "*.yaml" ./cf s3://$s3bkt/$env/cf
aws s3 sync --exclude "*" --include "*.yaml" ./k8s s3://$s3bkt/$env/k8s
aws s3 cp hc.yam s3://$s3bkt/$env/hc.yam
