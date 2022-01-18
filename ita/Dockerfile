

FROM golang:1.17
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-c -w -s -linkmode external -extldflags -static" -a main

# FROM scratch
from alpine:latest
COPY --from=0 /app /app
ENTRYPOINT ["/app/main"]
