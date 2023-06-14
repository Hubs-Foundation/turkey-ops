package handlers

import (
	"encoding/json"
	"main/internal"
	"net/http"
	"time"
)

func handleMultiClusterReq(w http.ResponseWriter, r *http.Request, cfg HCcfg) error {

	tStart := time.Now()

	internal.Logger.Debug("multi-cluster req, hcCfg.Region: " + cfg.Region)

	cfg.TurkeyJobReqMethod = r.Method
	cfg.TurkeyJobJobId = w.Header().Get("X-Request-Id")
	cfg.TurkeyJobCallback = internal.Cfg.TurkeyJobCallback

	msgBytes, _ := json.Marshal(cfg)

	err := internal.Cfg.Gcps.PubSub_PublishMsg(internal.Cfg.TurkeyJobsPubSubTopicName, msgBytes)
	if err != nil {
		return err
	}

	//wait for callback
	callback_arr, err := internal.Cfg.Redis.BLPop(60*time.Second, cfg.TurkeyJobJobId)
	if err != nil {
		internal.Logger.Sugar().Debugf("failed @ catching callback: %v", err)
	}
	callback_map := map[string]string{}
	_ = json.Unmarshal([]byte(callback_arr[0]), &callback_map)

	internal.Logger.Sugar().Debugf("callback: %v", callback_map)

	tElapsed := time.Since(tStart)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"_job_id":  cfg.TurkeyJobJobId,
		"_hub_id":  cfg.HubId,
		"job_id":   callback_map["id"],
		"hub_id":   callback_map["hub_id"],
		"domain":   callback_map["domain"],
		"tElapsed": tElapsed.Seconds(),
	})
	return nil
}
