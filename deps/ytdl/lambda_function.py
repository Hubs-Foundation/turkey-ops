import sys
import json

import youtube_dl
from youtube_dl.utils import std_headers, random_user_agent

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

def get_result(url, extra_params):
    std_headers['User-Agent'] = random_user_agent()
    for k, v in extra_params.items():
        if k in ALLOWED_EXTRA_PARAMS:
            convertf = ALLOWED_EXTRA_PARAMS[k]
            if convertf == bool:
                convertf = lambda x: query_bool(x, k)
            elif convertf == list:
                convertf = lambda x: x.split(',')
            extra_params[k] = convertf(v)
    res = get_videos(url, extra_params)
    return res

ALLOWED_EXTRA_PARAMS = {
    # 'format': str,
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
