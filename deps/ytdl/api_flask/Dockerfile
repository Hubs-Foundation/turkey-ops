FROM python:3.7.3-slim

RUN pip3 install youtube-dl flask gunicorn
COPY main.py .
#WORKDIR /app

cmd gunicorn main:app -w 2 --threads 2 -b 0.0.0.0:8080