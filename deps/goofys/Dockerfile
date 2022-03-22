FROM alpine
run apk add curl bash
workdir gcsfuse
run curl https://github.com/kahing/goofys/releases/latest/download/goofys -L --output goofys
run chmod +x goofys
# install catfs, required to run goofys with cache
RUN curl -L -O https://github.com/kahing/catfs/releases/download/v0.8.0/catfs && \
    mv catfs /usr/bin && chmod 0755 /usr/bin/catfs
copy entrypoint.sh entrypoint.sh
entrypoint bash entrypoint.sh