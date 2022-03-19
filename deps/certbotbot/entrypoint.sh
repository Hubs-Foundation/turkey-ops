
function need_new_cert(){    
    if kubectl -n ingress get secret letsencrypt -o=go-template='{{index .data "tls.crt"}}' | base64 -d > tls.crt; then return 0; fi
    ls -lha tls*
    if grep -q "$DOMAIN" <<< "$(openssl x509 -noout -subject -in tls.crt)"; then echo "bad cert CN -- need new cert"; return 0; fi
    if openssl x509 -checkend 2592000 -noout -in tls.crt; then echo "expiring -- need new cert";return 0; else return 1; fi
}

function get_new_cert(){
    echo "get_new_cert with DOMAIN=${DOMAIN}, EMAIL=$CERTBOT_EMAIL"
    certbot certonly --non-interactive --agree-tos -m $CERTBOT_EMAIL \
        --dns-$DNS_PROVIDER --dns-$DNS_PROVIDER-propagation-seconds 300 \
        --debug-challenges -d \*.$DOMAIN -d $DOMAIN
}

function get_kubectl(){
    echo "getting kubectl"
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
    chmod +x ./kubectl && mv ./kubectl /usr/local/bin

    echo "making in-cluster config for kubectl"
    kubectl config set-cluster the-cluster --server="https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}" --certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    kubectl config set-credentials pod-token --token="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"
    kubectl config set-context pod-context --cluster=the-cluster --user=pod-token
    kubectl config use-context pod-context
}

function save_cert(){
    kubectl -n ingress create secret tls letsencrypt \
        --cert=/etc/letsencrypt/live/${DOMAIN}/fullchain.pem \
        --key=/etc/letsencrypt/live/${DOMAIN}/privkey.pem \
        --save-config --dry-run=client -o yaml | kubectl apply -f -
    echo "new cert: "
    kubectl -n ingress describe secret letsencrypt
}

export DNS_PROVIDER=$1
echo $GCP_SA_KEY > GCP_SA_KEY.json
chmod 600 GCP_SA_KEY.json
export GOOGLE_APPLICATION_CREDENTIALS="GCP_SA_KEY.json"

get_kubectl
if ! need_new_cert; then echo "good cert, exit in 15 min"; sleep 900; exit 0; fi
echo "getting new cert"
if ! get_new_cert; then echo "ERROR failed to get new cert, exit in 15 min"; sleep 900; exit 1; fi
if ! save_cert; then echo "ERROR failed to save cert"; sleep 300; fi



kubectl -n ingress rollout restart deployment ingress-controller
