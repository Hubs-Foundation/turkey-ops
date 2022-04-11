FROM python:alpine

RUN pip3 install youtube-dl flask gunicorn requests redis google-cloud
COPY main.py .
#WORKDIR /app

cmd gunicorn main:app -w 1 --threads 2 -b 0.0.0.0:8080