
mkdir -p $MNT_DIR
gcsfuse --version
echo "$GCP_SA_KEY" > gcp_sa_key.json && chmod 600 gcp_sa_key.json
export GOOGLE_APPLICATION_CREDENTIALS="gcp_sa_key.json"
gcsfuse "$GCS_BUCKET" "$MNT_DIR"
