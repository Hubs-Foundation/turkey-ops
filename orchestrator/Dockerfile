FROM golang:1.17 as buildEnv
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-c -w -s -linkmode external -extldflags -static"

from alpine:latest
WORKDIR /app
COPY --from=buildEnv /app/. /app/.
ENTRYPOINT ["/app/main"]