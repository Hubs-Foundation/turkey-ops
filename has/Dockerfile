FROM golang:1.17 as buildEnv
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-c -w -s -linkmode external -extldflags -static"

from alpine/openssl as certr
workdir certs
run openssl genrsa -out ca.key 2048 && openssl req -new -x509 -days 36500 -key ca.key -subj "/CN=selfsigned" -out ca.crt
run openssl req -newkey rsa:2048 -nodes -keyout key.pem -subj "/CN=turkeyorch" -out server.csr
run printf "subjectAltName=DNS:turkeyorch,DNS:*.turkeyorch,DNS:turkeyorch.turkey-services.svc.cluster.local" > f
run openssl x509 -req -extfile f -days 36500 -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out cert.pem

from mozillareality/hubs:stable-latest as hubs
ENV turkeyCfg_thumbnail_server="nearspark.reticulum.io"
ENV turkeyCfg_base_assets_path="https://{{.Subdomain}}.assets.{{.Domain}}/hubs/"
ENV turkeyCfg_non_cors_proxy_domains="{{.Subdomain}}.{{.Domain}},{{.Subdomain}}.assets.{{.Domain}}"
ENV turkeyCfg_reticulum_server="{{.Subdomain}}.{{.HubDomain}}"
ENV turkeyCfg_cors_proxy_server="hubs-proxy.com"
ENV turkeyCfg_shortlink_domain="{{.Subdomain}}.{{.HubDomain}}"
ENV turkeyCfg_tier="{{.Tier}}"
ENV turkeyCfg_sentry_dsn="foobar"
run sed -i '/nginx/d' run.sh
run bash run.sh

from mozillareality/spoke:stable-latest as spoke
ENV turkeyCfg_thumbnail_server="nearspark.reticulum.io"
ENV turkeyCfg_base_assets_path="https://{{.Subdomain}}.assets.{{.Domain}}/hubs/"
ENV turkeyCfg_non_cors_proxy_domains="{{.Subdomain}}.{{.Domain}},{{.Subdomain}}.assets.{{.Domain}}"
ENV turkeyCfg_reticulum_server="{{.Subdomain}}.{{.HubDomain}}"
ENV turkeyCfg_cors_proxy_server="hubs-proxy.com"
ENV turkeyCfg_shortlink_domain="{{.Subdomain}}.{{.HubDomain}}"
ENV turkeyCfg_tier="{{.Tier}}"
ENV turkeyCfg_is_moz="false"
ENV turkeyCfg_sentry_dsn="foobar"
run sed -i '/nginx/d' run.sh
run bash run.sh

from alpine:latest
run apk add curl terraform
WORKDIR /app
COPY --from=buildEnv /app/. /app/.
copy --from=hubs /www/hubs/* ./_hubs/
copy --from=spoke /www/spoke/* ./_spoke/

copy --from=certr /certs .
ENTRYPOINT ["/app/main"]