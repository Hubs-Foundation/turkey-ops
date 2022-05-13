
#hello
FROM golang:1.17
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-c -w -s -linkmode external -extldflags -static" -a main

from alpine/openssl as certr
workdir certs
run openssl req -x509 -newkey rsa:2048 -sha256 -days 36500 -nodes -keyout key.pem -out cert.pem -subj '/CN=turkeyorch'

# FROM scratch
from alpine:latest
COPY --from=0 /app /app
copy --from=certr /certs .
ENTRYPOINT ["/app/main"]
