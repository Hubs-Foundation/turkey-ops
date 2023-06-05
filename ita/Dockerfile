

FROM golang:1.18
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-c -w -s -linkmode external -extldflags -static" -a main

from alpine/openssl as certr
workdir certs
run openssl genrsa -out ca.key 2048 && openssl req -new -x509 -days 36500 -key ca.key -subj "/CN=selfsigned" -out ca.crt
run openssl req -newkey rsa:2048 -nodes -keyout key.pem -subj "/CN=turkeyorch" -out server.csr
run printf "subjectAltName=DNS:turkeyorch,DNS:*.turkeyorch,DNS:turkeyorch.turkey-services.svc.cluster.local" > f
run openssl x509 -req -extfile f -days 36500 -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out cert.pem

from alpine:latest
run apk update
run apk add curl postgresql
COPY --from=0 /app/. /app/.
copy --from=certr /certs .
ENTRYPOINT ["/app/main"]
