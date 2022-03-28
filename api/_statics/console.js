
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


document.getElementById("hc_deploy").addEventListener("click", orcReq("POST", "/hc_instance"), false);
// function hc_deploy(){ orcReq("POST", "/hc_instance") }

document.getElementById("hc_get").addEventListener("click", orcReq("GET", "/hc_instance"), false);
// function hc_get(){ orcReq("GET", "/hc_instance") }

document.getElementById("hc_del").addEventListener("click", orcReq("DELETE", "/hc_instance"), false);
// function hc_del(){ orcReq("DELETE", "/hc_instance") }

document.getElementById("hc_pause").addEventListener("click", orcReq("PATCH", "/hc_instance?status=down"), false);
// function hc_pause(){ orcReq("PATCH", "/hc_instance?status=down") }

document.getElementById("hc_resume").addEventListener("click", orcReq("PATCH", "/hc_instance?status=up"), false);
// function hc_resume(){ orcReq("PATCH", "/hc_instance?status=up") }


document.getElementById("turkeyAws").addEventListener("click", orcReq("POST", "/tco_aws"), false);
// function turkeyAws(){ orcReq("POST", "/tco_aws") }

document.getElementById("turkeyGcp").addEventListener("click", orcReq("POST", "/tco_gcp"), false);
// function turkeyGcp(){ orcReq("POST", "/tco_gcp") }

document.getElementById("turkeyGcp_del").addEventListener("click", orcReq("DELETE", "/tco_gcp"), false);
// function turkeyGcp_del(){ orcReq("DELETE", "/tco_gcp") }



function orcReq(method, path) {
  cfg=document.getElementById("cfg").value
  var xhttp = new XMLHttpRequest(); res=""
  xhttp.onreadystatechange = function() {if (this.readyState == 4 && this.status == 200) {res = this.responseText;}};
  xhttp.open(method, path, true);
  xhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhttp.send(cfg);
}

//-------------------------
function getDomain()
{
    hostName = window.location.hostname
    return hostName.substring(hostName.lastIndexOf(".", hostName.lastIndexOf(".") - 1) + 1);
}