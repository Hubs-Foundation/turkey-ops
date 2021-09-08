FROM golang:1.17 as buildEnv
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-c -w -s -linkmode external -extldflags -static" > /dev/null

from alpine:latest
RUN apk add --no-cache \
        python3 \
        py3-pip \
    && pip3 install --upgrade pip \
    && pip3 install \
        awscli \
    && rm -rf /var/cache/apk/*
RUN aws --version   # Just to make sure its installed alright

WORKDIR /app
COPY --from=buildEnv /app/. /app/.
ENTRYPOINT ["/app/main"]