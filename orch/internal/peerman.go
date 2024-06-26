package internal

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

type PeerReport struct {
	Domain    string `json:"domain"`
	HubDomain string `json:"hubdomain"`

	Region     string `json:"region"`
	HC_count   int    `json:"hc_count"`
	T_unix_sec int64  `json:"t_unix_sec"`
	// Token      string `json:"token"`
}

type PeerMan struct {
	peerMap map[string]PeerReport
	Mu      sync.Mutex
}

func NewPeerMan() *PeerMan {
	pm := &PeerMan{
		peerMap: map[string]PeerReport{},
	}
	pm.download()
	pm.cleanup()
	pm.upload()
	pm.startSyncJob()
	return pm
}

const redisKey_peerBook = "turkeyorchPeerBook"

func (pm *PeerMan) GetPeerMap() map[string]PeerReport {
	pm.Mu.Lock()
	defer pm.Mu.Unlock()
	return pm.peerMap
}

func (pm *PeerMan) FindPeerDomain(region string) []PeerReport {
	peerReports := []PeerReport{}
	for domain, report := range pm.peerMap {
		if strings.HasPrefix(domain, region) {
			peerReports = peerReports_addBy_hcCnt(peerReports, report)
		}
	}
	return peerReports
}

func peerReports_addBy_hcCnt(reports []PeerReport, report PeerReport) []PeerReport {
	reports = append(reports, report)
	for i := len(reports) - 1; i > 0; i-- {
		if reports[i].HC_count < reports[i-1].HC_count {
			buf := reports[i-1]
			reports[i-1] = reports[i]
			reports[i] = buf
		}
	}
	return reports
}

func (pm *PeerMan) download() {
	mapStr, err := Cfg.Redis.Get(redisKey_peerBook)
	if err != nil {
		Logger.Sugar().Errorf("failed to get from redis: %v", err)
	}
	peerMap := map[string]PeerReport{}
	err = json.Unmarshal([]byte(mapStr), &peerMap)
	if err != nil {
		Logger.Error("failed to unmarshal: " + err.Error())
	}
	Logger.Sugar().Debugf("downloaded peerMap: %v", peerMap)
	pm.Mu.Lock()
	pm.peerMap = peerMap
	pm.Mu.Unlock()
}

func (pm *PeerMan) cleanup() {
	pm.Mu.Lock()
	defer pm.Mu.Unlock()
	_peerMap := map[string]PeerReport{}
	for k, v := range pm.peerMap {
		if time.Now().Unix()-v.T_unix_sec < 7200 {
			_peerMap[k] = v
		}
	}
	pm.peerMap = _peerMap
}

func (pm *PeerMan) upload() {
	pm.Mu.Lock()
	peerMap := pm.peerMap
	pm.Mu.Unlock()

	peerMapBytes, err := json.Marshal(peerMap)
	if err != nil {
		Logger.Error("failed to marshal peerMap: " + err.Error())
	}
	err = Cfg.Redis.Set(redisKey_peerBook, string(peerMapBytes))
	if err != nil {
		Logger.Error("failed @ Cfg.Redis.Set: " + err.Error())
	} else {
		Logger.Sugar().Debugf("uploaded: %v", string(peerMapBytes))
	}
}

func (pm *PeerMan) startSyncJob() {
	cronjob := NewCron("PeerManSyncJob", 5*time.Minute)
	cronjob.Load("PeerManSyncJob", Cronjob_PeerManSyncJob)
	cronjob.Start()
}

func (pm *PeerMan) UpdatePeerAndUpload(report PeerReport) {
	Logger.Sugar().Debugf("adding: %v", report)
	pm.download()
	pm.Mu.Lock()
	pm.peerMap[report.Domain] = PeerReport{
		Domain:     report.Domain,
		HubDomain:  report.HubDomain,
		Region:     report.Region,
		HC_count:   report.HC_count,
		T_unix_sec: report.T_unix_sec,
		// Token:      report.Token,
	}
	pm.Mu.Unlock()
	pm.upload()
}

func Cronjob_PeerManSyncJob(interval time.Duration) {
	Logger.Debug("hello from Cronjob_PeerManSyncJob")
	Cfg.PeerMan.download()
	Cfg.PeerMan.cleanup()
	Cfg.PeerMan.upload()
}
