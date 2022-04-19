FROM node:lts-buster
WORKDIR /app
RUN apt-get update && apt-get -y install libnss3 libexpat1

COPY package.json package-lock.json /app/
RUN npm install
RUN npm install express
# RUN npm install aws-sdk

COPY . /app/
#run chmod +x /app/entrypoint.sh
# CMD node app.js
#CMD /app/entrypoint.sh
cmd AWS_LAMBDA_FUNCTION_NAME="something" LAMBDA_TASK_ROOT="/app" node app.js
#&& sleep 3 && curl localhost:5000/run?host=gtan.myhubs.net&hub_sid=ZuGLiti
