
mkdir -p $MNT_DIR
gcsfuse --version
keyfile="/gcsfuse/gcp_sa_key.json"
echo "$GCP_SA_KEY" > $keyfile && chmod 600 $keyfile
export GOOGLE_APPLICATION_CREDENTIALS=$keyfile
gcsfuse --stat-cache-ttl 1h --type-cache-ttl 1h -o allow_other --foreground --implicit-dirs --only-dir $GCS_DIR "$GCS_BUCKET" "$MNT_DIR" 
