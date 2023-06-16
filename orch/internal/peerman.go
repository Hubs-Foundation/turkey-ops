package internal

import (
	"encoding/json"
	"math"
	"strings"
	"sync"
	"time"
)

type PeerReport struct {
	Domain string `json:"domain"`

	Region    string `json:"region"`
	HC_count  int    `json:"hc_count"`
	TimeStamp string `json:"time_stamp"`
	Token     string `json: "token"`
}

type PeerInfo struct {
	Region    string `json:"region"`
	HC_count  int    `json:"hc_count"`
	TimeStamp string `json:"time_stamp"`
	Token     string `json:"token`
}
type PeerMan struct {
	infoMap map[string]PeerInfo
	Mu      sync.Mutex
}

func NewPeerMan() *PeerMan {
	m := map[string]PeerInfo{}
	pm := &PeerMan{
		infoMap: m,
	}
	pm.download()
	pm.startSyncJob()
	return pm
}

const redisKey = "turkeyorchPeerBook"

func (pm *PeerMan) GetInfoMap() map[string]PeerInfo {
	pm.Mu.Lock()
	defer pm.Mu.Unlock()
	return pm.infoMap
}

func (pm *PeerMan) FindPeerDomain(region string) (string, string, int) {
	infoMap := pm.GetInfoMap()
	peerDomain := ""
	peerToken := ""
	peer_hc_cnt := math.MaxInt
	for domain, info := range infoMap {
		if strings.HasPrefix(domain, region) {
			if info.HC_count < peer_hc_cnt {
				peer_hc_cnt = info.HC_count
				peerDomain = domain
				peerToken = info.Token
			}
		}
	}
	return peerDomain, peerToken, peer_hc_cnt
}

func (pm *PeerMan) SetInfoMap(infoMap map[string]PeerInfo) {
	Logger.Sugar().Debugf("setting: %v", infoMap)
	pm.Mu.Lock()
	pm.infoMap = infoMap
	pm.Mu.Unlock()
}

func (pm *PeerMan) upload() {
	pm.Mu.Lock()
	infoMap := pm.infoMap
	pm.Mu.Unlock()

	infoMapBytes, err := json.Marshal(infoMap)
	if err != nil {
		Logger.Error("failed to marshal infoMap: " + err.Error())
	}
	err = Cfg.Redis.Set(redisKey, string(infoMapBytes))
	if err != nil {
		Logger.Error("failed @ Cfg.Redis.Set: " + err.Error())
	} else {
		Logger.Sugar().Debugf("uploaded: %v", string(infoMapBytes))
	}
}

func (pm *PeerMan) download() {
	mapStr, err := Cfg.Redis.Get(redisKey)
	if err != nil {
		Logger.Sugar().Errorf("failed to get from redis: %v", err)
	}
	infoMap := map[string]PeerInfo{}
	err = json.Unmarshal([]byte(mapStr), &infoMap)
	if err != nil {
		Logger.Error("failed to unmarshal: " + err.Error())
	}
	Logger.Sugar().Debugf("downloaded infoMap: %v", infoMap)
	Cfg.PeerMan.SetInfoMap(infoMap)
}

func (pm *PeerMan) startSyncJob() {
	cronjob := NewCron("PeerManSyncJob", 15*time.Minute)
	cronjob.Load("PeerManSyncJob", Cronjob_PeerManSyncJob)
	cronjob.Start()
}

func (pm *PeerMan) UpdatePeerAndUpload(report PeerReport) {
	Logger.Sugar().Debugf("adding: %v", report)
	pm.download()
	pm.Mu.Lock()
	pm.infoMap[report.Domain] = PeerInfo{
		Region:    report.Region,
		HC_count:  report.HC_count,
		TimeStamp: report.TimeStamp,
		Token:     report.Token,
	}
	pm.Mu.Unlock()
	pm.upload()
}

func Cronjob_PeerManSyncJob(interval time.Duration) {
	Logger.Debug("hello from Cronjob_PeerManSyncJob")
	Cfg.PeerMan.download()
}
