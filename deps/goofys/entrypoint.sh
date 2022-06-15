
./goofys --version

keyfile="/gcsfuse/gcp_sa_key.json"
echo "$GCP_SA_KEY" > $keyfile && chmod 600 $keyfile
export GOOGLE_APPLICATION_CREDENTIALS=$keyfile
gcsfuse "gs://$GCS_BUCKET/$GCS_DIR" "$MNT_DIR"
