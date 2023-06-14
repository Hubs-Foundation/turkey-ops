package handlers

import (
	"context"
	"encoding/json"
	"main/internal"
	"net/http"
	"time"
)

func handleMultiClusterReq(w http.ResponseWriter, r *http.Request, cfg HCcfg) error {
	internal.Logger.Debug("multi-cluster req, hcCfg.Region: " + cfg.Region)

	cfg.TurkeyJobReqMethod = r.Method
	cfg.TurkeyJobJobId = w.Header().Get("X-Request-Id")
	cfg.TurkeyJobCallback = internal.Cfg.TurkeyJobCallback

	msgBytes, _ := json.Marshal(cfg)

	err := internal.Cfg.Gcps.PubSub_PublishMsg(internal.Cfg.TurkeyJobsPubSubTopicName, msgBytes)
	if err != nil {
		return err
	}

	callback, err := internal.Cfg.Redis.BLPop(context.Background(), 0*time.Second, cfg.TurkeyJobJobId).Result()
	if err != nil {
		internal.Logger.Sugar().Debugf("failed @ catching callback: %v", err)
	}

	internal.Logger.Sugar().Debugf("callback: %v", callback)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id": cfg.TurkeyJobJobId,
		"hub_id": cfg.HubId,
	})
	return nil
}