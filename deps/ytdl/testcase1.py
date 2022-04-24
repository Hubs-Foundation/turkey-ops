from operator import truediv
from typing import get_type_hints
import requests, re, random, threading, numpy as np, time

#configs
ytdlHost="hubs-ytdl-fsu7tyt32a-uc.a.run.app"
userNum=3
userRampStepSec=2
durationSec=30
##############################################################
result = {}
resTime = []
stop_threads = False
def main():
    print('starting', userNum, 'users with duration=', durationSec, 'and ramp up interval=', userRampStepSec, 'sec')
    for userId in range(userNum):
        x = threading.Thread(target=vuser, args=(userId,))
        x.start()
        time.sleep(userRampStepSec)
    time.sleep(durationSec)
    global stop_threads; stop_threads = True
    x.join()
    # Print result
    print ('(http.200s) 50th percentile:', np.percentile(resTime,50))
    print ('(http.200s) 90th percentile:', np.percentile(resTime,90))
    print (r'http results {code: count}',result)

def getYtVids():
    yt_homepage=requests.get("https://www.youtube.com/").content.decode("utf8")  
    regPattern = re.compile('watch\?v=[A-Z,0-9,a-z,_,-]*') 
    ytPageResult = regPattern.findall (yt_homepage)
    return ytPageResult

def vuser(id):
    vids = getYtVids()
    print ('starting userId',id)
    while True:
        v = random.choice(vids)
        res = requests.get("https://"+ytdlHost+"/api/info?url=https://www.youtube.com/"+v)

        # if res.status_code not in result:
        #     result[res.status_code]=1
        # else:
        #     result[res.status_code] += 1
        try: result[res.status_code] += 1 
        except: result[res.status_code] = 1

        if res.status_code == 200:
            resTime.append(res.elapsed)
        global stop_threads
        if stop_threads:
            break
##############################################################
main()

