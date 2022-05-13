FROM golang:1.17 as buildEnv
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-c -w -s -linkmode external -extldflags -static"

from alpine/openssl as certr
workdir certs
run printf "[req]\ndistinguished_name=dn\n[dn]\nCN=turkeyorch\n[SAN]\nsubjectAltName=DNS:*.turkeyorch,DNS:turkeyorch.turkey-services.svc.cluster.local" > cfg
run openssl req -x509 -newkey rsa:2048 -sha256 -days 36500 -nodes -keyout key.pem -out cert.pem -subj '/CN=turkeyorch' -reqexts SAN -config cfg

from alpine:latest
run apk add curl jq
WORKDIR /app
COPY --from=buildEnv /app/. /app/.
run curl \
    $(curl -sL https://releases.hashicorp.com/terraform/index.json | jq -r '.versions[].builds[].url' | sort -t. -k 1,1n -k 2,2n -k 3,3n -k 4,4n | egrep -v 'rc|beta' | egrep 'linux.*amd64' |tail -1) \
    > ./_files/tf/terraform.zip \
    && cd ./_files/tf/ && unzip terraform.zip && rm terraform.zip    
copy --from=certr /certs .
ENTRYPOINT ["/app/main"]