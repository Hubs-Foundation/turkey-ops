package internal

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

func dumpHeader(r *http.Request) string {
	headerBytes, _ := json.Marshal(r.Header)
	return string(headerBytes)
}

func Healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&Healthy) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

var Ita_admin_info = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"ssh_totp_qr_data":     "N/A",
		"ses_max_24_hour_send": 99999,
		"using_ses":            true,
		"worker_domain":        "N/A",
		"assets_domain":        "N/A",
		"server_domain":        cfg.Domain,
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
