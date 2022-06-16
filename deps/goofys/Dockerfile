# ### alpine
# FROM alpine as buildenv
# RUN wget -O catfs https://github.com/kahing/catfs/releases/download/v0.8.0/catfs
# RUN apk add git go
# RUN git clone https://github.com/kahing/goofys
# RUN cd goofys && go mod tidy && go build -ldflags "-X main.Version=`git rev-parse HEAD`"

# FROM alpine
# RUN apk add fuse bash
# workdir goofys
# COPY --from=buildenv /catfs /usr/bin/catfs
# RUN chmod +x /usr/bin/catfs
# COPY --from=buildenv /goofys/goofys ./goofys
# RUN chmod +x goofys
# copy entrypoint.sh entrypoint.sh
# entrypoint bash entrypoint.sh


### debian
FROM golang:1.18-bullseye as buildenv
RUN apt-get update && apt-get install -y wget git golang
RUN wget -O catfs https://github.com/kahing/catfs/releases/download/v0.8.0/catfs
RUN git clone https://github.com/kahing/goofys
RUN cd goofys && go mod tidy && go build -ldflags "-X main.Version=`git rev-parse HEAD`"

FROM debian:bullseye
RUN apt-get update && \
    apt-get -y install --no-install-recommends \
            # gcsfuse dependencies \
            fuse \
            # why not including this, how many MB it saves?
            ca-certificates \
            # finally, clean up to make image smaller \
            && apt-get clean
# catfs is required to run goofys with cache
COPY --from=buildenv /go/catfs /usr/bin/catfs
RUN chmod 0755 /usr/bin/catfs
# get the binary
COPY --from=buildenv /go/goofys/goofys ./goofys
RUN chmod +x goofys
copy entrypoint.sh entrypoint.sh
entrypoint bash entrypoint.sh