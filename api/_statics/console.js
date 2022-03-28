
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


document.getElementById("hc_deploy").addEventListener("click", function(){orcReq("POST", "/hc_instance");}, false);
document.getElementById("hc_get").addEventListener("click", function(){orcReq("GET", "/hc_instance");}, false);
document.getElementById("hc_del").addEventListener("click", function(){orcReq("DELETE", "/hc_instance");}, false);
document.getElementById("hc_pause").addEventListener("click", function(){orcReq("PATCH", "/hc_instance?status=down");}, false);
document.getElementById("hc_resume").addEventListener("click", function(){orcReq("PATCH", "/hc_instance?status=up");}, false);

document.getElementById("turkeyAws").addEventListener("click", function(){orcReq("POST", "/tco_aws");}, false);
document.getElementById("turkeyGcp").addEventListener("click", function(){orcReq("POST", "/tco_gcp");}, false);
document.getElementById("turkeyGcp_del").addEventListener("click", function(){orcReq("DELETE", "/tco_gcp");}, false);

function orcReq(method, path) {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open("POST", "/hc_instance", true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}

//-------------------------
function getDomain()
{
    hostName = window.location.hostname
    return hostName.substring(hostName.lastIndexOf(".", hostName.lastIndexOf(".") - 1) + 1);
}