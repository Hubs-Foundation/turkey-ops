import sys, json, os, socket, errno
import youtube_dl
from youtube_dl.utils import std_headers, random_user_agent
from flask import Flask, request, jsonify
from requests import get

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

#########################################################################

app = Flask(__name__)

@app.route("/api/info")
def ytdl_api_info():
    print(' >>>>>> @/api/info, request.remote_addr: ' + request.remote_addr)
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
    return jsonify(result)

@app.route("/api/quit")
def ytdl_api_quit():
    print(' >>>>>> @/api/quit, request.remote_addr: ' + request.remote_addr)
    sys.exit(errno.EINTR)

@app.route("/aidiheaipi")
def ytdl_api_envvars():
    return jsonify(id + " @ " + ip)

### global init
ip = get('https://ipinfo.io/ip').content.decode('utf8')
id = get('http://metadata.google.internal/computeMetadata/v1/instance/id', headers={"Metadata-Flavor":"Google"}).content.decode('utf8')
print (' >>>>>> publicIP:' + id + " @ " + ip + ' <<<<<<')

### local debug only ... 
if __name__ == "__main__":
    # print(os.environ.items)
    print("hostname: "+socket.gethostname())
    # port = int(os.environ.get("PORT", 5000))
    # app.run(debug=True,host='0.0.0.0',port=port)


