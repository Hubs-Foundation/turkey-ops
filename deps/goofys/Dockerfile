FROM alpine
RUN apk add fuse bash
workdir goofys
RUN wget -O goofys https://github.com/kahing/goofys/releases/latest/download/goofys
RUN chmod +x goofys
copy entrypoint.sh entrypoint.sh
entrypoint bash entrypoint.sh