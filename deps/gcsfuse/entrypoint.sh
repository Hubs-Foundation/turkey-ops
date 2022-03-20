
mkdir -p $MNT_DIR
gcsfuse --version
keyfile="/gcsfuse/gcp_sa_key.json"
echo "$GCP_SA_KEY" > $keyfile && chmod 600 $keyfile
export GOOGLE_APPLICATION_CREDENTIALS=$keyfile
gcsfuse "$GCS_BUCKET" "$MNT_DIR" -o allow_other --foreground --implicit-dirs
