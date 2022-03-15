
# echo "certbotbot started, DOMAIN=${DOMAIN}, EMAIL=$CERTBOT_EMAIL"

# certbot certonly --non-interactive --agree-tos -m $EMAIL \
#     --dns-route53 --dns-route53-propagation-seconds 30 \
#     --debug-challenges -d \*.$DOMAIN -d $DOMAIN

# Certificate is saved at: /etc/letsencrypt/live/gcp.myhubs.net/fullchain.pem
# Key is saved at:         /etc/letsencrypt/live/gcp.myhubs.net/privkey.pem

echo "getting kubectl"
curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
chmod +x ./kubectl && mv ./kubectl /usr/local/bin

echo "making kube config"
kubectl config set-cluster the-cluster --server="https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}" --certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
kubectl config set-credentials pod-token --token="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"
kubectl config set-context pod-context --cluster=the-cluster --user=pod-token
kubectl config use-context pod-context

echo "testing kubectl"
kubectl get ns

