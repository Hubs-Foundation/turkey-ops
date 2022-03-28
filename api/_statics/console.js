
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




document.getElementById("hc_deploy").addEventListener("click", deployBtnClicked);
function deployBtnClicked() {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("POST", "/hc_instance", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}

document.getElementById("hc_get").addEventListener("click", getBtnClicked);
function getBtnClicked() {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("GET", "/hc_instance", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}

document.getElementById("hc_del").addEventListener("click", delClicked);
function delClicked() {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("DELETE", "/hc_instance", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}
document.getElementById("hc_pause").addEventListener("click", delClicked);
function delClicked() {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("PATCH", "/hc_instance?status=down", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}
document.getElementById("hc_resume").addEventListener("click", delClicked);
function delClicked() {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("PATCH", "/hc_instance?status=up", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}

//-----------
document.getElementById("turkeyAws").addEventListener("click", turkeyAwsBtnClicked);
function turkeyAwsBtnClicked() {
  cfg=document.getElementById("cluster_cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("POST", "/tco_aws", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}
//-----------
document.getElementById("turkeyGcp").addEventListener("click", turkeyGcpBtnClicked);
function turkeyGcpBtnClicked() {
  cfg=document.getElementById("cluster_cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("POST", "/tco_gcp", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}
//-----------
document.getElementById("turkeyGcp_del").addEventListener("click", turkeyGcpDelBtnClicked);
function turkeyGcpDelBtnClicked() {
  cfg=document.getElementById("cluster_cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("POST", "/tco_gcp_del", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}
//-------------------------
function getDomain()
{
    hostName = window.location.hostname
    return hostName.substring(hostName.lastIndexOf(".", hostName.lastIndexOf(".") - 1) + 1);
}