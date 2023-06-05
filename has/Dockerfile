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
run sed -i '/nginx/d' run.sh
run turkeyCfg_thumbnail_server="nearspark\.reticulum\.io" \
    turkeyCfg_base_assets_path="https://{{\.Subdomain}}\.assets\.{{\.Domain}}/hubs/" \
    turkeyCfg_non_cors_proxy_domains="{{\.Subdomain}}\.{{\.Domain}},{{\.Subdomain}}\.assets\.{{\.Domain}}" \
    turkeyCfg_reticulum_server="{{\.Subdomain}}\.{{\.HubDomain}}" \
    turkeyCfg_cors_proxy_server="hubs-proxy\.com" \
    turkeyCfg_shortlink_domain="{{\.Subdomain}}\.{{\.HubDomain}}" \
    turkeyCfg_tier="{{\.Tier}}" \
    turkeyCfg_sentry_dsn="foobar" \
    bash run.sh || true

from mozillareality/spoke:stable-latest as spoke
run sed -i '/nginx/d' run.sh
run turkeyCfg_thumbnail_server="nearspark\.reticulum\.io" \
    turkeyCfg_base_assets_path="https://{{\.Subdomain}}\.assets\.{{\.Domain}}/hubs/" \
    turkeyCfg_non_cors_proxy_domains="{{\.Subdomain}}\.{{\.Domain}},{{\.Subdomain}}\.assets\.{{\.Domain}}" \
    turkeyCfg_reticulum_server="{{\.Subdomain}}\.{{\.HubDomain}}" \
    turkeyCfg_cors_proxy_server="hubs-proxy\.com" \
    turkeyCfg_shortlink_domain="{{\.Subdomain}}\.{{\.HubDomain}}" \
    turkeyCfg_tier="{{\.Tier}}" \
    turkeyCfg_sentry_dsn="foobar" \
    turkeyCfg_is_moz = "false" \
    bash run.sh || true

from alpine:latest
run apk add curl terraform
WORKDIR /app
COPY --from=buildEnv /app/. /app/.
copy --from=hubs /www/hubs/* ./_hubs/
copy --from=spoke /www/spoke/* ./_spoke/

copy --from=certr /certs .
ENTRYPOINT ["/app/main"]