if [ -z $NAMESPACE ]; then echo "namespace unspecified, defaulting to <ingress>" && export NAMESPACE="ingress"; fi
echo "NAMESPACE=$NAMESPACE"
echo "DOMAIN=$DOMAIN"
echo "HUB_DOMAIN=$HUB_DOMAIN"
echo "CHALLENGE=$CHALLENGE"
echo "CERTBOT_EMAIL=$CERTBOT_EMAIL"

function need_new_cert(){    
    if kubectl -n $NAMESPACE get secret letsencrypt -o=go-template='{{index .data "tls.crt"}}' | base64 -d > tls.crt; then return 0; fi
    ls -lha tls*
    if grep -q "$DOMAIN" <<< "$(openssl x509 -noout -subject -in tls.crt)"; then echo "bad cert CN -- need new cert"; return 0; fi
    if openssl x509 -checkend 2592000 -noout -in tls.crt; then echo "expiring -- need new cert";return 0; else return 1; fi
}

function get_new_cert_dns(){
    echo "get_new_cert_dns with DOMAIN=${DOMAIN}, EMAIL=$CERTBOT_EMAIL"
    certbot certonly --non-interactive --agree-tos -m $CERTBOT_EMAIL \
        --dns-$CHALLENGE --dns-$CHALLENGE-propagation-seconds 300 \
        --debug-challenges -d $DOMAIN -d \*.$DOMAIN -d \*.stream.$DOMAIN -d $HUB_DOMAIN -d \*.$HUB_DOMAIN -d \*.stream.$HUB_DOMAIN
}

function get_new_cert_http(){
    echo "get_new_cert_http -- requires $DOMAIN/.well-known/acme-challenge* routed into this pod"
    echo "start nginx and wait 120 sec for ingress to pick up the pod" && nginx && sleep 120
    certbot certonly --non-interactive --agree-tos -m $CERTBOT_EMAIL --preferred-challenges http --nginx -d $DOMAIN
    if [ "$?" -ne 0 ]; then
      echo "try #1 failed, retry in 300 sec ..." && sleep 300
      certbot certonly --non-interactive --agree-tos -m $CERTBOT_EMAIL --preferred-challenges http --nginx -d $DOMAIN
    fi
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
    name=$1
    kubectl -n $NAMESPACE create secret tls $name \
        --cert=/etc/letsencrypt/live/${DOMAIN}/fullchain.pem \
        --key=/etc/letsencrypt/live/${DOMAIN}/privkey.pem \
        --save-config --dry-run=client -o yaml | kubectl apply -f -
    echo "new cert: "
    kubectl -n $NAMESPACE describe secret $name
}

export CHALLENGE=$1
echo $GCP_SA_KEY > GCP_SA_KEY.json
chmod 600 GCP_SA_KEY.json
export GOOGLE_APPLICATION_CREDENTIALS="GCP_SA_KEY.json"

get_kubectl
kubectl -n $NAMESPACE patch cronjob certbotbot -p '{"spec":{"schedule": "0 0 */13 * *"}}'
if [ "$?" -ne 0 ]; then echo "ERROR -- can't patch cronjob, wtb rbac permision fixes"; sleep 3600; exit 1; fi

if ! need_new_cert; then echo "good cert, exit in 15 min"; sleep 900; exit 0; fi

echo "getting new cert"
if [ "$CHALLENGE" = "http" ]; then
  get_new_cert_http
else
  get_new_cert_dns
fi

if [ "$?" -ne 0 ]; then echo "ERROR failed to get new cert, exit in 15 min"; sleep 900; exit 1; fi

echo "saving new cert"
if ! save_cert "letsencrypt-$CHALLENGE"; then echo "ERROR failed to save cert"; sleep 300;exit 1; fi

if [ "$NAMESPACE" == "ingress" ]; then kubectl -n $NAMESPACE rollout restart deployment haproxy; fi

if ! [[ $? ]]; then echo "[ERROR],[certbotbot],wtb manual help pls"; sleep 36000; fi

