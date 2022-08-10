// const runbot = require("./run-bot");
var express = require('express');

var app = express();

app.get('/_healthz', function (req, res) {
  res.send('1');
});

var exec = require('child_process').exec;

app.get('/run-bot', function (req, res) {
    console.log(req.query)
    console.log("handling");
  
    // const url = `https://${host}/${hub_sid}${lobby ? "" : "?bot=true"}`;
  
    cmd="node run-bot.js -u "+req.query.url+" -a bot-recording.mp3 -d bot-recording.json"
    console.log("cmd: ", cmd)


    try {
    //   await run(url, duration, !!lobby, !!audio, !!slow);
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