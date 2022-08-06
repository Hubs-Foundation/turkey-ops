FROM alpine:3.15
ARG VERSION=1.17.0

RUN apk add -U --no-cache --upgrade busybox && \
  apk add -U --no-cache autoconf autoconf-doc automake udns-dev curl gcc libc-dev libevent-dev libtool make \
  libevent udns openssl-dev pkgconfig postgresql-client libpq
  
run curl -o  /tmp/pgbouncer-$VERSION.tar.gz -L https://pgbouncer.github.io/downloads/files/$VERSION/pgbouncer-$VERSION.tar.gz && \
  cd /tmp && tar xvfz /tmp/pgbouncer-$VERSION.tar.gz && cd pgbouncer-$VERSION && ./configure --prefix=/usr --with-udns && \
  make && cp pgbouncer /usr/bin && mkdir -p /etc/pgbouncer /var/log/pgbouncer /var/run/pgbouncer && \
  cp etc/pgbouncer.ini /etc/pgbouncer/pgbouncer.ini.example && cp etc/userlist.txt /etc/pgbouncer/userlist.txt.example && \
  touch /etc/pgbouncer/userlist.txt 

run cd /tmp && rm -rf /tmp/pgbouncer*  && \
  apk del --purge autoconf autoconf-doc automake udns-dev curl gcc libc-dev libevent-dev libtool make libressl-dev pkgconfig

COPY entrypoint.sh /entrypoint.sh
run chmod +x ./entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/bin/pgbouncer", "/etc/pgbouncer/pgbouncer.ini"]
