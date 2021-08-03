FROM golang:1.13.9 as buildEnv
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-c -w -s -linkmode external -extldflags -static"

#gettting bio
WORKDIR /app/_tools
run export org="biome-sh"; export repo="biome"; \
    ver=$(curl -s https://api.github.com/repos/$org/$repo/releases/latest \
    | grep "tag_name" | awk '{print substr($2, 2, length($2)-3)}'); \
    export dl="https://github.com/$org/$repo/releases/download/$ver/bio-$ver-x86_64-linux.tar.gz"; \
    echo "[info] getting bio from: $dl"; curl -L -o ./bio.gz $dl; tar -xf ./bio.gz; \
    ./bio --version   

from alpine:latest
WORKDIR /app
COPY --from=buildEnv /app/. /app/.
ENTRYPOINT ["/app/main"]

# FROM alpine:latest as certs
# RUN apk --update add ca-certificates

# FROM scratch
# ENV PATH=/bin
# COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# WORKDIR /app/_files
# COPY --from=buildEnv /app/_files/. /app/_files/.

# WORKDIR /app/_templates
# COPY --from=buildEnv /app/_templates/. /app/_templates/.

# WORKDIR /app/_tools
# COPY --from=buildEnv /app/_tools/. /app/_tools/.

# WORKDIR /app
# COPY --from=buildEnv /app/. /app/.
# ENTRYPOINT ["/app/main"]



