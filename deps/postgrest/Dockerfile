# from postgrest/postgrest:v9.0.1
FROM ubuntu:xenial

ARG ARCH=x64
ARG POSTGREST_VERSION=v9.0.1

# Install libpq5
RUN apt-get -qq update && apt-get -qq install -y libpq5 curl ca-certificates xz-utils && \
    apt-get -qq clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

run export file=postgrest-${POSTGREST_VERSION}-linux-static-${ARCH}.tar.xz && cd /tmp && \
    curl -SLO https://github.com/PostgREST/postgrest/releases/download/${POSTGREST_VERSION}/$file && \
    ls -lha $file && tar -xJvf $file && mv postgrest /usr/local/bin/postgrest

CMD /usr/local/bin/postgrest
