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
	token := r.Header.Get("Token")
	if token == "" || !internal.TokenBook.CheckToken(token) {
		internal.Logger.Debug("bad token: " + token)
		http.Error(w, "", http.StatusNotFound)
		return
	}
	// internal.Logger.Debug("good token: " + token)

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
		"result": "[remote] done",
		"job_id": cfg.TurkeyJobJobId,
		"hub_id": cfg.HubId,
		"region": internal.Cfg.Region,
		"domain": internal.Cfg.HubDomain,
	})

})
