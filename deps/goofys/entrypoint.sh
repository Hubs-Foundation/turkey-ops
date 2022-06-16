
./goofys --version

keyfile="gcp_sa_key.json"
echo "$GCP_SA_KEY" > $keyfile && chmod 600 $keyfile
GOOGLE_APPLICATION_CREDENTIALS=$keyfile ./goofys -f "gs://$GCS_BUCKET/$GCS_DIR" "$MNT_DIR"
