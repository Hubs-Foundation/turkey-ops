package handlers

import (
	"encoding/json"
	"fmt"
	"main/internal"
	"net/http"
)

var Ita_admin_info = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"ssh_totp_qr_data":     "N/A",
		"ses_max_24_hour_send": 99999,
		"using_ses":            true,
		"worker_domain":        "N/A",
		"assets_domain":        "N/A",
		"server_domain":        internal.Cfg.Domain,
		"provider":             "N/A",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	return
})

var Ita_cfg_ret_ps = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	return
})

var HC_launch_fallback = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hc_launch_fallback" || r.Method != "GET" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	internal.GetLogger().Debug(dumpHeader(r))

	fmt.Fprintf(w, "wip")
	return
})
