from tanfarming/ret
WORKDIR /hab/svc/reticulum/files
cmd touch ssl.key&&touch ssl.pem&&touch ssl-chain.pem
copy ./certs .
entrypoint bash /init.sh run mozillareality/reticulum
