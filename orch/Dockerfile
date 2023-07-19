FROM golang:1.20 as buildEnv
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-c -w -s -linkmode external -extldflags -static"

from alpine/openssl as certr
workdir certs
run openssl genrsa -out ca.key 2048 && openssl req -new -x509 -days 36500 -key ca.key -subj "/CN=selfsigned" -out ca.crt
run openssl req -newkey rsa:2048 -nodes -keyout key.pem -subj "/CN=turkeyorch" -out server.csr
run printf "subjectAltName=DNS:turkeyorch,DNS:*.turkeyorch,DNS:turkeyorch.turkey-services.svc.cluster.local" > f
run openssl x509 -req -extfile f -days 36500 -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out cert.pem

from alpine:latest
run apk add curl terraform postgresql
WORKDIR /app
COPY --from=buildEnv /app/. /app/.
# run curl \
#     $(curl -sL https://releases.hashicorp.com/terraform/index.json | jq -r '.versions[].builds[].url' | sort -t. -k 1,1n -k 2,2n -k 3,3n -k 4,4n | egrep -v 'rc|beta' | egrep 'linux.*amd64' |tail -1) \
#     > ./_files/tf/terraform.zip \
#     && cd ./_files/tf/ && unzip terraform.zip && rm terraform.zip    
copy --from=certr /certs .
ENTRYPOINT ["/app/main"]