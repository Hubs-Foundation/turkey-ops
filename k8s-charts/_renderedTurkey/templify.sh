#!/bin/bash

yam="../../orchestrator/_files/turkey.yam"

cp hc-dev0.yaml $yam
sed -i 's/dev0/{{.Subdomain}}/g' $yam
sed -i 's/dev_gimmechart/{{.UserId}}/g' $yam
sed -i 's/myhubs.net/{{.Domain}}/g' $yam



