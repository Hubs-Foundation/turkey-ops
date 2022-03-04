
s3fs --version
echo "$AWS_KEY:$AWS_SECRET" >> s3fsPasswd && chmod 600 s3fsPasswd
s3fs "$S3_BUCKET" "$MNT_DIR" -o passwd_file=s3fsPasswd && tail -f /dev/null
