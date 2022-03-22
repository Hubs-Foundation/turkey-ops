
mkdir -p $MNT_DIR
export cacheDir="/home/goofys-cache"
mkdir -p 
./goofys --version
keyfile="/gcsfuse/gcp_sa_key.json"
echo "$GCP_SA_KEY" > $keyfile && chmod 600 $keyfile
export GOOGLE_APPLICATION_CREDENTIALS=$keyfile
./goofys -f --cache $cacheDir -o allow_other "gs://$GCS_BUCKET" "$MNT_DIR" 

