# FROM python:slim-bullseye

# # RUN pip3 install google-api-python-client
# RUN pip3 install google-cloud-run google-cloud-logging
# RUN pip3 install youtube-dl flask gunicorn requests redis
# COPY main.py .
# #WORKDIR /app

# cmd gunicorn main:app -w 1 --threads 2 -b 0.0.0.0:8080

FROM python:slim-bullseye

WORKDIR /app

RUN pip3 install google-cloud-run google-cloud-logging
RUN pip3 install flask gunicorn requests redis

# run pip3 install youtube-dl
run apt update && apt install -y git && \ 
    git clone https://github.com/ytdl-org/youtube-dl.git && \
    cd youtube-dl
COPY main.py /app/youtube-dl/main.py

cmd gunicorn main:app -w 1 --threads 2 -b 0.0.0.0:8080