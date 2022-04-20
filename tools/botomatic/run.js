// single autonomous virtual user 

const lambda = require("./index");

lambda.handler(
    {query: {
        hub_sid: process.env.hub_sid,
        host: process.env.host,
        duration: process.env.duration || 30,
        lobby: process.env.lobby,
        audio: process.env.audio,
        slow: process.env.slow,
    }}, 
    null,
    async function (something, callback){
        console.log("callback: ", callback)
        res.status(callback.statusCode).header(callback.headers).send(callback.body)
    }
)

