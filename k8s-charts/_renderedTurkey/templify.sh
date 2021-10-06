#!/bin/bash

yam="../../api/_files/turkey.yam"
# yam="./test.yam"

cp hc-dev0.yaml $yam
# sed -i 's/ret_dev0/{{.DBname}}/g' $yam
sed -i 's/dev0/{{.Subdomain}}/g' $yam
sed -i 's/someString/{{.TurkeyId}}/g' $yam
sed -i 's/myhubs.net/{{.Domain}}/g' $yam
sed -i 's/gtan@mozilla.com/{{.UserEmail}}/g' $yam
sed -i 's#  PERMS_KEY: ----.*#  PERMS_KEY: {{.PermsKey}}#g' $yam
sed -i 's#".*3RY0qLmdthY6Q0RZ4oyNQSL035BmYLNdleX1qVpG1zfQeLWf.*#"{{.JWK}}"#g' $yam


