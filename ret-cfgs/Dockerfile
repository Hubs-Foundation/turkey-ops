from alpine/openssl:latest as builder
workdir /certs
run openssl req -x509 -newkey rsa:4096 -sha256 -days 36500 -nodes -keyout ssl.key -out ssl.pem -subj '/CN=hubs'
run cp ssl.pem ssl-chain.pem

from dummy1/reticulum
WORKDIR /hab/svc/reticulum/files
copy --from=builder /certs .

run chown -R hab:hab ./* && ls -lha
# run echo 'hostname ${alb_name}' > /entrypoint.sh
# run echo 'bash /init.sh run mozillareality/reticulum' >> /entrypoint.sh
# run cat /entrypoint.sh
# entrypoint bash /entrypoint.sh
entrypoint bash /init.sh run mozillareality/reticulum
