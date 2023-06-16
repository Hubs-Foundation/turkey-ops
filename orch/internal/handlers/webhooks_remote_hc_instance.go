package handlers

import (
	"encoding/json"
	"main/internal"
	"net/http"
)

var Webhook_remote_hc_instance = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/webhooks/remote_hc_instance" {
		http.Error(w, "", http.StatusNotFound)
		return
	}
	cfg, err := getHcCfg(r)
	if err != nil {
		http.Error(w, "", http.StatusNotFound)
		return
	}
	//handing hc_instance request
	err = handle_hc_instance_req(r, cfg)
	if err != nil {
		internal.Logger.Sugar().Errorf("failed @ handle_hc_instance_req: %v", err)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"result":    "[remote] done",
		"hub_id":    cfg.HubId,
		"region":    cfg.Region,
		"domain":    cfg.HubDomain,
		"subdomain": cfg.Subdomain,
	})

})
