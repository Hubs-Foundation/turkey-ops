
mkdir -p $MNT_DIR
gcsfuse --version

gcsfuse "$GCS_BUCKET" "$MNT_DIR"
