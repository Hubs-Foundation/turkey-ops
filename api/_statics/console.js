
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


document.getElementById("hc_deploy").addEventListener("click", hc_deploy, false);
function hc_deploy(){ orcReq("POST", "/hc_instance","cfg") }

document.getElementById("hc_get").addEventListener("click", hc_get, false);
function hc_get(){ orcReq("GET", "/hc_instance","cfg") }

document.getElementById("hc_del").addEventListener("click", hc_del, false);
function hc_del(){ orcReq("DELETE", "/hc_instance","cfg") }

document.getElementById("hc_pause").addEventListener("click", hc_pause, false);
function hc_pause(){ orcReq("PATCH", "/hc_instance?status=down","cfg") }

document.getElementById("hc_resume").addEventListener("click", hc_resume, false);
function hc_resume(){ orcReq("PATCH", "/hc_instance?status=up","cfg") }


document.getElementById("turkeyAws").addEventListener("click", turkeyAws, false);
function turkeyAws(){ orcReq("POST", "/tco_aws","cluster_cfg") }

document.getElementById("turkeyGcp").addEventListener("click", turkeyGcp, false);
function turkeyGcp(){ orcReq("POST", "/tco_gcp","cluster_cfg") }

document.getElementById("turkeyGcp_del").addEventListener("click", turkeyGcp_del, false);
function turkeyGcp_del(){ orcReq("DELETE", "/tco_gcp","cluster_cfg") }



function orcReq(method, path, cfgBoxId) {
  cfg=document.getElementById(cfgBoxId).value
  divLogBoard=document.getElementById("divLogBoard");
  var xhr = new XMLHttpRequest(); res=""
  xhr.onreadystatechange = function() {if (this.readyState == 4) {
    res = "res = http"+this.status + ":"+ this.responseText;
    divLogBoard.innerHTML+=res +"<br>";
    divLogBoard.scrollTop = divLogBoard.scrollHeight;
  }};
  xhr.open(method, path, true);
  xhr.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhr.send(cfg);
}

//-------------------------
function getDomain()
{
    hostName = window.location.hostname
    return hostName.substring(hostName.lastIndexOf(".", hostName.lastIndexOf(".") - 1) + 1);
}