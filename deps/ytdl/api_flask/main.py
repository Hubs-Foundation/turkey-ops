import youtube_dl, sys, json, os, socket, requests, redis, logging, random, base64, time
# import google.auth, google.auth.transport.requests
# from google.oauth2 import service_account
from google.cloud import run_v2
import google.cloud.logging
from youtube_dl.utils import std_headers, random_user_agent
from flask import Flask, request, jsonify
from datetime import datetime



################################################################################################## 
####################### ytdl helpers, got junks, todo: clean it up ###############################
##################################################################################################

def lambda_handler(event, context):
    # event['url']="http://whatever/?url=https://www.youtube.com/watch?v=zjMuIxRvygQ&moreparams=values"
    # logging.debug"Received event: " + json.dumps(event, indent=2))
    params=event['url'].split('?',1)[1]
    ytdl_params=dict(item.split('=',1) for item in params.split('&'))
    ytdl_url=ytdl_params['url']
    debugMsg="***no issue***"
    try:
        result = get_result(ytdl_url, ytdl_params)
    except:
        ytdl_params.pop("format",None)
        result = get_result(ytdl_url, ytdl_params)
        debugMsg="***retried with [format] dropped***"
    return {
        'url': ytdl_url,
        'info': result,
        'debugMsg': debugMsg
    }    
    # result = get_result(ytdl_url, ytdl_params)     
    # return {
    #     'url': ytdl_url,
    #     'info': result,
    # }

class SimpleYDL(youtube_dl.YoutubeDL):
    def __init__(self, *args, **kargs):
        super(SimpleYDL, self).__init__(*args, **kargs)
        self.add_default_info_extractors()

def get_videos(url, extra_params):
    '''
    Get a list with a dict for every video founded
    '''
    ydl_params = {
        'format': 'best',
        'cachedir': False,
        # 'logger': current_app.logger.getChild('youtube-dl'),
        # 'proxy': current_app.config['proxy'],
    }
    ydl_params.update(extra_params)
    ydl = SimpleYDL(ydl_params)
    res = ydl.extract_info(url, download=False)
    return res

class WrongParameterTypeError(ValueError):
    def __init__(self, value, type, parameter):
        message = '"{}" expects a {}, got "{}"'.format(parameter, type, value)
        super(WrongParameterTypeError, self).__init__(message)

def query_bool(value, name, default=None):
    if value is None:
        return default
    value = value.lower()
    if value == 'true':
        return True
    elif value == 'false':
        return False
    else:
        raise WrongParameterTypeError(value, 'bool', name)
        
def get_result():
    url = request.args['url']
    extra_params = {}

    std_headers['User-Agent'] = random_user_agent()
    for k, v in request.args.items():
        if k == "user_agent":
            std_headers['User-Agent'] = v
        else:
            if k in ALLOWED_EXTRA_PARAMS:
                convertf = ALLOWED_EXTRA_PARAMS[k]
                if convertf == bool:
                    convertf = lambda x: query_bool(x, k)
                elif convertf == list:
                    convertf = lambda x: x.split(',')
                extra_params[k] = convertf(v)
    res = get_videos(url, extra_params)
    return res
# def get_result(url, extra_params):
#     std_headers['User-Agent'] = random_user_agent()
#     for k, v in extra_params.items():
#         if k in ALLOWED_EXTRA_PARAMS:
#             convertf = ALLOWED_EXTRA_PARAMS[k]
#             if convertf == bool:
#                 convertf = lambda x: query_bool(x, k)
#             elif convertf == list:
#                 convertf = lambda x: x.split(',')
#             extra_params[k] = convertf(v)
#     res = get_videos(url, extra_params)
#     return res

ALLOWED_EXTRA_PARAMS = {
    'format': str,
    'playliststart': int,
    'playlistend': int,
    'playlist_items': str,
    'playlistreverse': bool,
    'matchtitle': str,
    'rejecttitle': str,
    'writesubtitles': bool,
    'writeautomaticsub': bool,
    'allsubtitles': bool,
    'subtitlesformat': str,
    'subtitleslangs': list,
}
def flatten_result(result):
    r_type = result.get('_type', 'video')
    if r_type == 'video':
        videos = [result]
    elif r_type == 'playlist':
        videos = []
        for entry in result['entries']:
            videos.extend(flatten_result(entry))
    elif r_type == 'compat_list':
        videos = []
        for r in result['entries']:
            videos.extend(flatten_result(r))
    return videos

################################################################################################## 
##################################### turkey funcs ###############################################
##################################################################################################

def cloudrun_rollout_restart():

    rkey_last_f=rkey+":lastReset_time"
    cooldown = 600 # seconds

    t0=float(0)
    t=redis_client.get(rkey_last_f)
    if t is not None:
        t0=float(t)

    dt = time.time() - float(t0)
    if dt < cooldown:
        return "skipped -- in cooldown -- need: " + str(cooldown) + " has: "+str(dt)

    # req=run_v2.GetServiceRequest(name=SVC_NAME_FULL)
    # svc=client.get_service(request=req)
    # request = run_v2.UpdateServiceRequest(service=svc)
    # res = client.update_service(request=request)

    knativeBase="https://us-central1-run.googleapis.com/apis/serving.knative.dev/v1/"

    getSvcUrl=knativeBase+"namespaces/{}/services/{}".format(projectId, svcName)

    inst_sa_token_res = getGcpMetadata(metadataUrl+"instance/service-accounts/"+full_sa+"/token")
    inst_sa_token=json.loads(inst_sa_token_res)['access_token']

    res=requests.get(getSvcUrl, headers={"Authorization":"Bearer "+inst_sa_token})
    print("cloudrun_rollout_restart~~~get_knative_res.text: " + res.text)
    reqJson=json.loads(res.text)

    ###
    for condition in reqJson["status"]["conditions"]:
        status=str(condition["status"])        
        if status != "True":
            return "skipped -- not ready for new revision (already in progress?)"
    ###

    revisionName=svcName + "-" + datetime.today().strftime("%Y%m%d%H%M%S")

    args = {
        'ServiceName':svcName, 
        'revisionName':revisionName,         
        'projectId':reqJson["metadata"]["namespace"], 
        'vpcConn':reqJson["spec"]["template"]["metadata"]["annotations"]["run.googleapis.com/vpc-access-connector"],
        'sa':reqJson["spec"]["template"]["spec"]["serviceAccountName"], 
        'image':reqJson["spec"]["template"]["spec"]["containers"][0]["image"],
        'redeploy_at': str(redeploy_at)
        }

    logging.debug(args)
    
    knativeJsonStr='''
    {{"apiVersion": "serving.knative.dev/v1",
    "kind": "Service",
    "metadata": {{"name": "{ServiceName}","namespace": "{projectId}"}},
    "spec": {{
        "template": {{
        "metadata": {{
            "name": "{revisionName}",
            "annotations": {{
                "run.googleapis.com/vpc-access-egress": "private-ranges-only",
                "autoscaling.knative.dev/minScale": "1",
                "autoscaling.knative.dev/maxScale": "100",
                "run.googleapis.com/vpc-access-connector": "{vpcConn}"}}}},
        "spec": {{
            "serviceAccountName": "{sa}",
            "containers": [{{
                "image": "{image}",
                "resources": {{ "limits":{{ "memory":"2Gi", "cpu":"2000m" }} }},
                "env": [
                    {{"name": "dummy","value": "dummy"}},
                    {{"name": "REDEPLOY_AT","value": "{redeploy_at}"}}
                    ]}}]}}}}}}}}
    '''.format(**args)

    # logging.debug(" >>>>>> knativeJsonStr: \n"+knativeJsonStr)

    res=requests.put(
        knativeBase+"namespaces/{}/services/{}".format(projectId, svcName), 
        headers={"Content-type":"application/json", "Authorization":"Bearer "+inst_sa_token,},
        json=json.loads(knativeJsonStr))

    # logging.debug(" >>>>>> put-res.text"+res.text)
    if res.status_code !=200:
        logging.warning("problem @ cloudrun_rollout_restart, res.status_code: "+str(res.status_code) + ", body: " + res.text)
    else:
        redis_client.set(rkey_last_f, time.time())

    return str(res.text)

def toInt(num):
    return int(num)

def getGcpMetadata(url):
    try:
        val=requests.get(url, headers={"Metadata-Flavor":"Google"}).content.decode('utf8')
        logging.info("getGcpMetadata -- received "+ str(len(val)) + "chars for "+url)
        return val
    except Exception as e:
        logging.error("getGcpMetadata -- failed for url: "+url + "error="+str(e))
    return "" 

################################################################################################## 
########################################## routes ################################################
##################################################################################################

app = Flask(__name__)

@app.route("/api/info")
def ytdl_api_info():
    url = request.args['url']

    result = get_result()
    key = 'info'
    if query_bool(request.args.get('flatten'), 'flatten', False):
        result = flatten_result(result)
        key = 'videos'
    result = {
        'url': url,
        key: result,
    }
    redis_client.zincrby(rkey, 1, inst_ip)
    
    #update ip usage count, redeploy at high usage
    cnt = redis_client.zscore(rkey, inst_ip)
    # logging.debug( "NOT redeploying -- " + inst_ip + ", cnt=" + str(cnt)+ ", redeploy_at="+ str(redeploy_at))
    if cnt >=redeploy_at :
        logging.warning( "redeploying -- " + inst_ip + " with cnt=" + str(cnt)+ " exceeded "+ str(redeploy_at))
        r=cloudrun_rollout_restart()
        logging.debug("and cloudrun_rollout_restart says: " + str(r))

    return jsonify(result)

@app.route("/api/stats")
def ytdl_api_stats():
    _inst_ip_cnt = redis_client.zscore(rkey, inst_ip)
    report={
        # "ytdl_cache_stats_hit": redis_client.get("ytdl_cache_stats_hit").decode('utf-8'),
        # "ytdl_cache_stats_miss": redis_client.get("ytdl_cache_stats_miss").decode('utf-8'),
        "_rkey": rkey,
        "_inst_ip": inst_ip,
        "_inst_ip_cnt": _inst_ip_cnt
        }
    
    # top_stat =redis_client.zrevrange(rkey, 0,0, withscores=True)
    
    stats=redis_client.zrevrange(rkey, 0, -1, withscores=True)

    if len(stats)>0:
        report["_top_ip"] = str(stats[0][0])
        report["_top_cnt"] = str(stats[0][1])
        for ip, cnt in stats:
            report[str(cnt)]=str(ip)
    
    return jsonify(report)

@app.route("/api/force-new-revision")
def ytdl_api_rrtest():
    r=cloudrun_rollout_restart()
    return str(r)
################################################################################################## 
########################################### init #################################################
##################################################################################################
mode=set()
try:
    gcpLoggingClient=google.cloud.logging.Client()
    gcpLoggingClient.setup_logging()
    mode.add("gcp")
    logging.info("mode + gcp")
except Exception as e:
    logging.info("gcp logging failed to init" + str(e))

metadataUrl=os.environ.get('metadataUrl', "http://metadata.google.internal/computeMetadata/v1/")
logging.debug("metadataUrl="+metadataUrl)

svcName=os.environ.get("svcName","hubs-ytdl")
full_sa=os.environ.get("fullSaNAme", "hubs-ytdl@hubs-dev-333333.iam.gserviceaccount.com")
inst_ip = requests.get('https://ipinfo.io/ip').content.decode('utf8')
redeploy_at = int(os.environ.get('REDEPLOY_AT', 88))

projectId=getGcpMetadata(metadataUrl+"project/project-id")
inst_id = getGcpMetadata(metadataUrl+"instance/id")


redis_client = redis.StrictRedis(
    host=os.environ.get('REDIS_HOST', '10.208.38.179'), 
    port=int(os.environ.get('REDIS_PORT', 6379)))
rkey = "ytdl:"+ datetime.today().strftime("%Y%m%d")

try:
    redis_client.expire(rkey, 604800)   # a week
    mode.add("redis")
    logging.debug("mode + redis")
except:
    logging.warn("bad redis config")

logging.debug(" @@@@@@ IP: "+inst_ip +", rkey: " + rkey +", hostname: " + socket.gethostname() + ", id: " + inst_id)

sys.stdout.flush()

################################################################################################## 
##################################### local debug only ###########################################
##################################################################################################
if __name__ == "__main__":
    # logging.debug(os.environ.items)
    # os.environ.setdefault('SERVICE_NAME', 'hubs-ytdl')
    # cloudrun_rollout_restart()
    port = int(os.environ.get("PORT", 5000))
    app.run(debug=True,host='0.0.0.0',port=port)


