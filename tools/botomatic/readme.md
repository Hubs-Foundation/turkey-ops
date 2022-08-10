

### a docker wrap of:
https://github.com/mozilla/hubs/tree/master/scripts/bot

### sample usage 
#### hosted
`curl https://botomatic-fsu7tyt32a-uc.a.run.app/run-bot?url=https://smoke-hubs.mozilla.com/0zuesf6c6mf`

#### local
`docker build -t boto . && docker run -it -p 5000:5000 -e BOTO_LOCAL=1 boto`

and then

`curl https://localhost:5000/run-bot?url=https://smoke-hubs.mozilla.com/0zuesf6c6mf`
