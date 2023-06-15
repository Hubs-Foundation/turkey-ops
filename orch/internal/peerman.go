package internal

import (
	"encoding/json"
	"sync"
	"time"
)

type PeerReport struct {
	Region   string `json:"region"`
	Domain   string `json:"domain"`
	HC_count int    `json:"hc_count"`
}

type PeerInfo struct {
	Region   string `json:"region"`
	HC_count int    `json:"hc_count"`
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
	pm.startSyncJob()
	return pm
}

const redisKey = "turkeyorchPeerBook"

func (pm *PeerMan) GetInfoMap() map[string]PeerInfo {
	pm.Mu.Lock()
	defer pm.Mu.Unlock()
	return pm.infoMap
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
	Cfg.Redis.Set(redisKey, string(infoMapBytes))
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
	pm.download()

	pm.Mu.Lock()
	pm.infoMap[report.Domain] = PeerInfo{
		Region:   report.Region,
		HC_count: report.HC_count,
	}
	pm.Mu.Unlock()

	pm.upload()
}

func Cronjob_PeerManSyncJob(interval time.Duration) {
	Logger.Debug("hello from Cronjob_PeerManSyncJob")
	Cfg.PeerMan.download()
}
