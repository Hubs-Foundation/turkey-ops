
FROM python:slim-bullseye

RUN pip3 install google-cloud-run google-cloud-logging
RUN pip3 install flask gunicorn requests redis

# run pip3 install youtube-dl
run apt update && apt install -y git && \ 
    git clone https://github.com/ytdl-org/youtube-dl.git && \
    mv youtube-dl/youtube_dl/ /youtube_dl/
COPY main.py .
run mv /youtube-dl/youtube_dl/ /youtube_dl/

cmd gunicorn main:app -w 1 --threads 2 -b 0.0.0.0:8080