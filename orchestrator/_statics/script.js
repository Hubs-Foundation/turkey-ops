
//============================== =====inits=================================
document.getElementById("AWSEx").addEventListener("click", AWSExClicked);
document.getElementById("AWSBtn").addEventListener("click", AWSBtnClicked);
document.getElementById("K8sEx").addEventListener("click", K8sExClicked);
document.getElementById("K8sBtn").addEventListener("click", K8sBtnClicked);

window.addEventListener("load", streamLogs);


// $(document).ready(function(){
//   $('[data-toggle="onebtndep-popover"]').popover({
//     title: "json like this",
//     content: `
// {<br />
//   "awsKey":    "$AWS_ACCESS_KEY_ID",<br />
//   "awsSecret": "$AWS_SECRET_ACCESS_KEY",<br />
//   "awsRegion": "$AWS_DEFAULT_REGION",<br />
//   "cf_CFparameters": "CF parameter overrides",<br />
// }<br />
// `,
//     html:true,
//     trigger:"focus",
//   });   
// });

//====================================funcs=================================
function streamLogs(){
  var source = new EventSource("/LogStream");
  divLogBoard=document.getElementById("divLogBoard");
  source.onmessage = function (event) {
    // console.warn(event.data)
    divLogBoard.innerHTML+=event.data +"<br>";
    divLogBoard.scrollTop = divLogBoard.scrollHeight;
  }
}


function AWSExClicked(){
  document.getElementById("AWSInput").value = 
`{
    "awsKey":    "$AWS_ACCESS_KEY_ID",
    "awsSecret": "$AWS_SECRET_ACCESS_KEY",
    "awsRegion": "$AWS_DEFAULT_REGION",
    "deploymentName": "(optional) used as a prefix on the stack name, default == t",
    "cf_<CFparameterName>": "(optional, 0 or more) CF input parameter override",
}`
}
function AWSBtnClicked() {
    input=document.getElementById("AWSInput").value
    var xhttp = new XMLHttpRequest(); res=""
    xhttp.onreadystatechange = function() {
      if (this.readyState == 4 && this.status == 200) {
        res = this.responseText;
      }
    };
    xhttp.open("POST", "/TurkeyDeployAWS", true);
    xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
    xhttp.send(input);
  }

  function K8sExClicked(){
  document.getElementById("kubeCfg").value = `
{
  "hello":    "world"
}`
}

function K8sBtnClicked() {
  subdomain=document.getElementById("subdomain").value
  kubeCfg=document.getElementById("kubeCfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {
    if (this.readyState == 4 && this.status == 200) {
      res = this.responseText;
    }
  };
  xhttp.open("POST", "/TurkeyDeployK8s?subdomain="+subdomain, true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(kubeCfg);
}

// function streamLogs(){
// var source = new EventSource("/LogStream");
// divLogBoard=document.getElementById("divLogBoard");
// source.onmessage = function (event) {
//   // console.warn(event.data)
//   divLogBoard.innerHTML+=event.data +"<br>";
//   divLogBoard.scrollTop = divLogBoard.scrollHeight;
// }
// }