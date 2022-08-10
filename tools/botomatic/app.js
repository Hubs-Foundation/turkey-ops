// const runbot = require("./run-bot");
var express = require('express');

var app = express();

app.get('/_healthz', function (req, res) {
  res.send('1');
});

let exec = require('child_process').execSync;
if (process.env.BOTO_LOCAL){
  console.log(" ### local mode ###")
  exec = require('child_process').exec
}

app.get('/run-bot', function (req, res) {
    console.log(req.query)
    console.log("handling");  
    cmd="node run-bot.js -u "+req.query.url+" -a bot-recording.mp3 -d bot-recording.json"
    console.log("cmd: ", cmd)
    try {      
      exec(cmd,        
        function (error, stdout, stderr) {
            console.log('stdout: ' + stdout);
            console.log('stderr: ' + stderr);
            if (error !== null) {
                console.log('exec error: ' + error);
            }
        });
    } catch (error) {
        res.send('failed: ${error}')
    }
    res.send('ok')
  });

app.listen(5000, function () {
  console.log('listening on :5000');
});