
// window.addEventListener("load", streamLogs);
// function streamLogs(){
//   var source = new EventSource("/LogStream");
//   divLogBoard=document.getElementById("divLogBoard");
//   source.onmessage = function (event) {
//     divLogBoard.innerHTML+=event.data +"<br>";
//     divLogBoard.scrollTop = divLogBoard.scrollHeight;
//   }
// }

window.addEventListener("load", logToken);
function logToken(){  
  document.getElementById("divLogBoard").innerHTML+=getCookie("_turkeyauthtoken") +"<br>"
}
function getCookie(name) {
  const cookies = `; ${document.cookie}`;
  console.log("cookies: ", cookies)
  const parts = cookies.split(`; ${name}=`);
  for (let part of parts){
    console.log("cookie: ", part)
  }
  if (parts.length === 2) return parts.pop().split(';').shift();
  return "ok"
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

// document.getElementById("hc_get").addEventListener("click", hc_get, false);
// function hc_get(){ orcReq("GET", "/hc_instance","cfg") }

document.getElementById("hc_del").addEventListener("click", hc_del, false);
function hc_del(){ orcReq("DELETE", "/hc_instance","cfg") }


document.getElementById("hc_patch").addEventListener("click", hc_patch, false);
function hc_patch(){ 
  orcReq("PATCH", "/hc_instance","cfg") 
}

document.getElementById("hc_pause").addEventListener("click", hc_pause, false);
function hc_pause(){ orcReq("PATCH", "/hc_instance?status=down","cfg") }

document.getElementById("hc_resume").addEventListener("click", hc_resume, false);
function hc_resume(){ orcReq("PATCH", "/hc_instance?status=up","cfg") }

document.getElementById("turkeyGcp_deploy").addEventListener("click", turkeyGcp_deploy, false);
function turkeyGcp_deploy(){ orcReq("POST", "/tco_gcp","cluster_cfg") }

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


document.getElementById("turkeyGcp_get").addEventListener("click", turkeyGcp_get, false);
function turkeyGcp_get(){ 
  tbody=document.getElementById("gcp_cluster_table").getElementsByTagName("tbody")[0];
  var xhr = new XMLHttpRequest(); res=""
  xhr.onreadystatechange = function() {if (this.readyState == 4) {
    var resJson = JSON.parse(this.responseText);
    console.log("this.responseText: ", this.responseText)
    console.log("resJson.clusters: ", resJson.clusters)
    tbody.innerHTML=resJson.clusters.map(row => `<tr><td>${row.name}</td><td><a href=${row.cfgbkt}>${row.cfgbkt}</a></td></tr>`).join('');
  }};
  xhr.open("GET", "/tco_gcp", true);
  // xhr.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhr.send();
}


document.getElementById("turkeyGcp_update_tf").addEventListener("click", turkeyGcp_update, false);
function turkeyGcp_update(){ 
  var mbody=document.getElementById("reviewUpdateModalBody");
  mbody.innerHTML="---loading---"
  var xhr = new XMLHttpRequest(); res=""
  xhr.onreadystatechange = function() {if (this.readyState == 4) {
    resJson = JSON.parse(this.responseText);
    console.log(resJson.stackName, resJson.msg, resJson.output)
    mbody.innerHTML=`<h1>${resJson.stackName}</h1><br><h2>${resJson.msg}</h2><br><h3>output: </h3><br>`+resJson.output;
  }};
  xhr.open("PATCH", "/tco_gcp?comp=tf", false);
  // xhr.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xhr.send(document.getElementById("cluster_cfg").value);

  // orcReq("PATCH", "/tco_gcp","cluster_cfg")
}

//-------------------------
function getDomain()
{
    hostName = window.location.hostname
    return hostName.substring(hostName.lastIndexOf(".", hostName.lastIndexOf(".") - 1) + 1);
}
//-------------------------

var table = document.getElementById("gcp_cluster_table");
var tbody = table.getElementsByTagName("tbody")[0];
tbody.onclick = function (e) {
  e = e || window.event;
  // var data = [];
  var target = e.srcElement || e.target;
  while (target && target.nodeName !== "TR") {
      target = target.parentNode;
  }
  if (target) {
      var cells = target.getElementsByTagName("td");
      // for (var i = 0; i < cells.length; i++) {
      //     data.push(cells[i].innerHTML);
      // }
      var clusterName=cells[0].innerHTML
      document.getElementById("cluster_cfg").value = `{
  "region":"us-central1",  
  "stackname":"` + clusterName + `"
}`
  }
};

document.getElementById("btn_sampleCfg_minDev").onclick = function(){
  document.getElementById("cluster_cfg").value = `{
  "domain":"changeMe.myhubs.dev",
  "hubdomain":"changeMe.myhubs.net",
  "region":"us-central1"
}`
}

document.getElementById("btn_sampleCfg_minProd").onclick = function(){
  document.getElementById("cluster_cfg").value = `{
  "domain":"changeMe.myhubs.dev",
  "hubdomain":"changeMe.myhubs.net",
  "region":"us-central1",
  "env":"prod",
  "OAUTH_CLIENT_ID_FXA":"changeMe",
  "OAUTH_CLIENT_SECRET_FXA":"changeMe"
}`
}

document.getElementById("btn_sampleCfg_fullBlown").onclick = function(){
  document.getElementById("cluster_cfg").value = `{
  "domain":"changeMe.myhubs.dev",
  "hubdomain":"changeMe.myhubs.net",
  "region":"us-central1",
  "env":"prod",
  "OAUTH_CLIENT_ID_FXA":"changeMe",
  "OAUTH_CLIENT_SECRET_FXA":"changeMe",
  "SMTP_SERVER":"changeMe",
  "SMTP_PORT":"changeMe",
  "SMTP_USER":"changeMe",
  "SMTP_PASS":"changeMe",
  "GCP_SA_KEY_b64":"changeMe",
  "GCP_SA_HMAC_KEY":"changeMe",
  "GCP_SA_HMAC_SECRET":"changeMe",
  "AWS_KEY":"changeMe",
  "AWS_SECRET":"changeMe",
  "SKETCHFAB_API_KEY":"changeMe",
  "ItaChan":"changeMe",
  "CLOUD":"changeMe",
  "DeploymentPrefix":"changeMe",
  "DeploymentId":"changeMe",
  "AWS_Ingress_Cert_ARN":"changeMe",
  "Options":"changeMe"
}`
}

document.getElementById("btn_sampleCfg_tandemDev").onclick = function(){
  document.getElementById("cluster_cfg").value = `{
  "domain":"changeMe.myhubs.dev",
  "hubdomain":"changeMe.myhubs.net",
  "region":"us-central1",
  "env":"dev",
  "VPC":"devmyhubsnet5hmnp"
}`
}

document.getElementById("btn_sampleCfg_tandemProd").onclick = function(){
  document.getElementById("cluster_cfg").value = `{
  "domain":"changeMe.myhubs.dev",
  "hubdomain":"changeMe.myhubs.net",
  "region":"us-central1",
  "env":"prod",
  "VPC":"myhubsdevacn55",
  "DASHBOARD_ACCESS_KEY":"<COPY>"
}`
}


document.getElementById("downgrade_cfg").onclick = function(){

  document.getElementById("cfg").value = `{
    "hub_id": "changeMe",
    "tier": "p0",
    "ccu_limit": "10",
    "storage_limit": "0.5",
    "subdomain": "changeMe"
}`
}