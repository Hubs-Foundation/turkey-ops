
window.addEventListener("load", streamLogs);
function streamLogs(){
  var source = new EventSource("/LogStream");
  divLogBoard=document.getElementById("divLogBoard");
  source.onmessage = function (event) {
    divLogBoard.innerHTML+=event.data +"<br>";
    divLogBoard.scrollTop = divLogBoard.scrollHeight;
  }
}

document.getElementById("login_google").addEventListener("click", login_google);
function login_google() {
  // window.location.replace("https://auth."+getDomain()+"/login?idp=google");
  window.location.replace("https://auth.myhubs.net/login?idp=google");
}

document.getElementById("login_fxa").addEventListener("click", login_fxa);
function login_fxa() {
  window.location.replace("https://auth."+getDomain()+"/login?idp=fxa");
}

document.getElementById("cfgEx_get").addEventListener("click", cfgEx_getClicked);
function cfgEx_getClicked(){
  document.getElementById("cfg").value = `{
  "userid": "user1"
}`
}

document.getElementById("cfgEx_deploy").addEventListener("click", cfgEx_deployClicked);
function cfgEx_deployClicked(){
  document.getElementById("cfg").value = `{
  "userid": "user1",
  "subdomain": "subdomain1",
  "domain": "myhubs.net"
}`
}

document.getElementById("deployBtn").addEventListener("click", deployBtnClicked);
function deployBtnClicked() {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("POST", "/orchestrator", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}

document.getElementById("getBtn").addEventListener("click", getBtnClicked);
function getBtnClicked() {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("GET", "/orchestrator", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}


//-------------------------
function getDomain()
{
    hostName = window.location.hostname
    return hostName.substring(hostName.lastIndexOf(".", hostName.lastIndexOf(".") - 1) + 1);
}