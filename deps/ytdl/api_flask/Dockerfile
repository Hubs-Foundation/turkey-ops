FROM python:slim-bullseye

RUN pip3 install google-api-python-client
RUN pip3 install google-cloud-run
RUN pip3 install youtube-dl flask gunicorn requests redis
COPY main.py .
#WORKDIR /app

cmd gunicorn main:app -w 1 --threads 2 -b 0.0.0.0:8080