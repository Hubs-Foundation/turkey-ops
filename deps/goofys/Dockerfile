# from golang:alpine AS builder
# ARG GCSFUSE_VERSION=0.40.0
# ENV GO111MODULE=off
# RUN apk --update --no-cache add git fuse fuse-dev
# RUN go get -d github.com/googlecloudplatform/gcsfuse
# RUN go install github.com/googlecloudplatform/gcsfuse/tools/build_gcsfuse
# RUN build_gcsfuse ${GOPATH}/src/github.com/googlecloudplatform/gcsfuse /tmp ${GCSFUSE_VERSION}

# FROM alpine
# RUN apk add --update --no-cache bash ca-certificates fuse
# COPY --from=builder /tmp/bin/gcsfuse /usr/bin
# COPY --from=builder /tmp/sbin/mount.gcsfuse /usr/sbin
# RUN ln -s /usr/sbin/mount.gcsfuse /usr/sbin/mount.fuse.gcsfuse
# workdir gcsfuse
# copy entrypoint.sh entrypoint.sh
# entrypoint bash entrypoint.sh

FROM alpine
workdir goofys
RUN apk add fuse
RUN wget https://github.com/kahing/goofys/releases/latest/download/goofys > goofys && chmod +x goofys
copy entrypoint.sh entrypoint.sh
entrypoint bash entrypoint.sh