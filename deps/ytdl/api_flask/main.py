import youtube_dl, sys, json, os, socket, requests, redis, logging
# import google.auth, google.auth.transport.requests
# from google.oauth2 import service_account
from google.cloud import run_v2
import google.cloud.logging
from youtube_dl.utils import std_headers, random_user_agent
from flask import Flask, request, jsonify
from datetime import datetime


def lambda_handler(event, context):
    # event['url']="http://whatever/?url=https://www.youtube.com/watch?v=zjMuIxRvygQ&moreparams=values"
    # print("Received event: " + json.dumps(event, indent=2))
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

def cloudrun_rollout_restart():
    # if SVC_NAME_FULL=="":
    #     raise ValueError('env var SERVICE_NAME is required to create new revision')
    # client = run_v2.ServicesClient()

    # req=run_v2.GetServiceRequest(name=SVC_NAME_FULL)
    # svc=client.get_service(request=req)
    # request = run_v2.UpdateServiceRequest(service=svc)
    # res = client.update_service(request=request)

    getSvcUrl="https://us-central1-run.googleapis.com/apis/serving.knative.dev/v1/namespaces/{}/services/{}".format(PROJECT_ID, svcName)
    print("getSvcUrl: " + getSvcUrl)
    res=requests.get(
        getSvcUrl, 
        headers={ "Authorization":"Bearer "+inst_sa_token} )    
    print(res.json)
    print("res.text: "+res.text)

    sys.stdout.flush()

    # res=requests.put(
    #     "https://us-central1-run.googleapis.com/apis/serving.knative.dev/v1/namespaces/{}/services/{}".format(PROJECT_ID, svcName), 
    #     headers={
    #         "Content-type":"application/json",
    #         "Authorization":"Bearer "+inst_sa_token,
    #         },
    #     # json=knative_json,
    #     ).content.decode('utf8')
    # logging.warning(res)
    # print(res)
    return res.json

def toInt(num):
    return int(num)

def getGcpMetadata(url):
    try:
        return requests.get(url, headers={"Metadata-Flavor":"Google"}).content.decode('utf8')
    except:
        logging.error("getGcpMetadata failed for url: "+url)        
    return "" 
#########################################################################

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

    top_stat =redis_client.zrevrange(rkey, 0,-1, withscores=True)
    top_ip=str(top_stat[0][0])
    top_cnt=int(top_stat[0][1])
    if top_cnt >=redeploy_at:
        logging.warning( "starting redeployment because "+top_ip + " with cnt="+top_cnt+" exceeded " + str(redeploy_at))

    return jsonify(result)

@app.route("/api/stats")
def ytdl_api_stats():
    report={
        "_rkey": rkey,
        "_inst_ip": inst_ip,
        }
    
    # top_stat =redis_client.zrevrange(rkey, 0,0, withscores=True)
    
    stats=redis_client.zrevrange(rkey, 0, -1, withscores=True)

    if len(stats)>0:
        report["_top_ip"] = str(stats[0][0])
        report["_top_cnt"] = str(stats[0][1])
        for ip, cnt in stats:
            report[str(cnt)]=str(ip)
    
    return jsonify(report)

@app.route("/api/rrtest")
def ytdl_api_rrtest():
    r=cloudrun_rollout_restart()
    return str(r)

################################################# init
try:
    google.cloud.logging.Client().setup_logging()
except:
    logging.warning("gcp logging failed to init")

METADATA_URL="http://metadata.google.internal/computeMetadata/v1/"
PROJECT_ID=os.environ.get("PROJECT_ID","hubs-dev-333333")

svcName="hubs-ytdl"

# inst_sa_token_res = getGcpMetadata(METADATA_URL+"instance/service-accounts/default/token")
full_sa="hubs-ytdl@hubs-dev-333333.iam.gserviceaccount.com"
inst_sa_token_res = getGcpMetadata(METADATA_URL+"instance/service-accounts/"+full_sa+"/token")

inst_sa_token=json.loads(inst_sa_token_res)['access_token']
logging.debug(" @@@@@@@ inst_sa_token: "+ inst_sa_token)
print(" >>>>>>>> inst_sa_token: "+ inst_sa_token)

inst_ip = requests.get('https://ipinfo.io/ip').content.decode('utf8')
inst_id = getGcpMetadata(METADATA_URL+"instance/id")

redeploy_at = int(os.environ.get('REDEPLOY_AT', 4500))

redis_client = redis.StrictRedis(
    host=os.environ.get('REDIS_HOST', '10.208.38.179'), 
    port=int(os.environ.get('REDIS_PORT', 6379)))

rkey = "ytdl:"+ datetime.today().strftime("%Y%m%d")

# redis_client.expire(rkey, 604800)   # a week
try:
    redis_client.expire(rkey, 604800)   # a week
except:
    logging.error("bad redis")

logging.debug(" @@@@@@ IP: "+inst_ip +", rkey: " + rkey +", hostname: " + socket.gethostname() + ", id: " + inst_id)
print(" >>>>>> IP: "+inst_ip +", rkey: " + rkey +", hostname: " + socket.gethostname() + ", id: " + inst_id)

sys.stdout.flush()


### local debug only ... 
if __name__ == "__main__":
    # print(os.environ.items)
    # os.environ.setdefault('SERVICE_NAME', 'hubs-ytdl')
    # cloudrun_rollout_restart()
    port = int(os.environ.get("PORT", 5000))
    app.run(debug=True,host='0.0.0.0',port=port)


