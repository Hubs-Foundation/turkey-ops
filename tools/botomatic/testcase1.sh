userCnt=5
stepWaitSec=3
durationSec=$(( $userCnt * $stepWaitSec * 3 ))
botomatic_host="https://botomatic-fsu7tyt32a-uc.a.run.app"
hub_host="nasa.myhubs.net"
hub_sid="vjwfCec"
url="$botomatic_host/run?host=$hub_host&hub_sid=$hub_sid&audio=true&duration=$durationSec"
for i in $(seq 1 $userCnt); do sleep $stepWaitSec; printf "\r > starting user # $i"; curl -s $url & done