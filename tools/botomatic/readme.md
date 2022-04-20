




### sample usage 
#### server (app.js)
sample invocation
###### single: 
`curl "https://botomatic-fsu7tyt32a-uc.a.run.app/run?host=gtan.myhubs.net&hub_sid=ZuGLiti&audio=true&duration=300"`
###### multiple
```
userCnt=25
duration=1200
url="https://botomatic-fsu7tyt32a-uc.a.run.app/run?host=gtan.myhubs.net&hub_sid=ZuGLiti&audio=true&duration=$duration"
for i in {1..$userCnt}; do curl -s $url; done
```
#### local (run.js)
###### build: `sed 's/app.js/run.js/g' ./Dockerfile > tmp && docker build -f tmp -t botorun .`
###### run `sudo docker run -e host=gtan.myhubs.net -e hub_sid=ZuGLiti -e duration=60 -e audio=true botorun`
