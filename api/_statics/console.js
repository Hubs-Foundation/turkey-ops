
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
  window.location.assign("https://auth."+getDomain()+"/login?idp=google&client="+window.location.origin+window.location.pathname);
}

document.getElementById("login_fxa").addEventListener("click", login_fxa);
function login_fxa() {
  window.location.assign("https://auth."+getDomain()+"/login?idp=fxa&client="+window.location.origin+window.location.pathname);
}

document.getElementById("logout").addEventListener("click", logout);
function logout() {
  window.location.assign("https://auth."+getDomain()+"/logout");
}

// document.getElementById("cfgEx_get").addEventListener("click", cfgEx_getClicked);
// function cfgEx_getClicked(){
//   document.getElementById("cfg").value = `{
//   "turkeyid": "someString"
// }`
// }

// document.getElementById("cfgEx_del").addEventListener("click", cfgEx_delClicked);
// function cfgEx_delClicked(){
//   document.getElementById("cfg").value = `{
//   "turkeyid": "someString",
//   "subdomain": "changeMe"
// }`
// }

// document.getElementById("cfgEx_deploy").addEventListener("click", cfgEx_deployClicked);
// function cfgEx_deployClicked(){
//   document.getElementById("cfg").value = `{
//   "turkeyid": "someString",
//   "subdomain": "changeMe"
// }`
// }

document.getElementById("deployBtn").addEventListener("click", deployBtnClicked);
function deployBtnClicked() {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("POST", "/hc_deploy", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}

document.getElementById("getBtn").addEventListener("click", getBtnClicked);
function getBtnClicked() {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("POST", "/hc_get", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}

document.getElementById("delBtn").addEventListener("click", delNsClicked);
function delNsClicked() {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("POST", "/hc_del", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}

// document.getElementById("delDbBtn").addEventListener("click", delDbClicked);
// function delDbClicked() {
//   cfg=document.getElementById("cfg").value
//   var xhttp = new XMLHttpRequest(); res=""
//   xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
//   xhttp.open("POST", "/hc_delDB", true);
//   xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
//   xhttp.send(cfg);
// }
//-----------
document.getElementById("turkeyAws").addEventListener("click", getBtnClicked);
function getBtnClicked() {
  cfg=document.getElementById("cluster_cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("POST", "/tco_aws", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}
//-----------
document.getElementById("turkeyGcp").addEventListener("click", getBtnClicked);
function getBtnClicked() {
  cfg=document.getElementById("cluster_cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("POST", "/tco_gcp", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}

//-------------------------
function getDomain()
{
    hostName = window.location.hostname
    return hostName.substring(hostName.lastIndexOf(".", hostName.lastIndexOf(".") - 1) + 1);
}