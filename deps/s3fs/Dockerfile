from alpine:3.15

env MNT_DIR "/var/s3fs"
ARG S3FS_VERSION=v1.90

run mkdir -p mntDir
# run apk --update --no-cache add s3fs-fuse
RUN apk --update --no-cache add fuse alpine-sdk automake autoconf libxml2-dev fuse-dev curl-dev git bash; \
    git clone https://github.com/s3fs-fuse/s3fs-fuse.git; \
    cd s3fs-fuse; \
    git checkout tags/${S3FS_VERSION}; \
    ./autogen.sh; \
    ./configure --prefix=/usr; \
    make; \
    make install; \
    make clean; \
    rm -rf /var/cache/apk/*; \
    apk del git automake autoconf;
workdir s3fs

copy entrypoint.sh entrypoint.sh
entrypoint bash entrypoint.sh
