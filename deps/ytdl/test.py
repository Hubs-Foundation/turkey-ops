from operator import truediv
from typing import get_type_hints
import requests, re, random, threading, numpy as np, time


def getYtVids():
    yt_homepage=requests.get("https://www.youtube.com/").content.decode("utf8")  
    regPattern = re.compile('watch\?v=[A-Z,0-9,a-z,_,-]*') 
    ytPageResult = regPattern.findall (yt_homepage)

    return ytPageResult

def vuser(id):
    vids = getYtVids()
    print (id,'ready')
    while True:
        v = random.choice(vids)
        res = requests.get(host + "/api/info?url=https://www.youtube.com/"+v)
        if res.status_code == 200:
            resTime.append(res.elapsed)
        if res.status_code == 500:
            resErr.append(v)
        global stop_threads
        if stop_threads:
            break


host = "https://hubs-ytdl-fsu7tyt32a-uc.a.run.app"
userCnt = 1
print("running", userCnt, "users")

resTime = []
resErr = []
stop_threads = False
for i in range(userCnt):
    x = threading.Thread(target=vuser, args=(i,))
    x.start()
time.sleep(100)
stop_threads = True
x.join()
# Print result
print (len(resTime),'of http200 reqeusts, 90th percentile response time', np.percentile(resTime,90))
print(len(resErr),'of http500 reqeusts, error url:')
print(*resErr, sep="\n")