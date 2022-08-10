

### a docker wrap of:
https://github.com/mozilla/hubs/tree/master/scripts/bot

### sample usage 
#### hosted
`curl "https://botomatic-fsu7tyt32a-uc.a.run.app/run-bot?url=https://gtan.myhubs.net/ZuGLiti"`

#### local
`docker build -t boto . && docker run -it -p 5000:5000 boto`

and then

`sudo docker run -e host=gtan.myhubs.net -e hub_sid=ZuGLiti -e duration=60 -e audio=true botorun`
