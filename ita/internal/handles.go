package internal

import (
	"encoding/json"
	"fmt"
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

// var HC_launch_fallback = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 	if r.URL.Path != "/hc_launch_fallback" || r.Method != "GET" {
// 		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 		return
// 	}
// 	Logger.Debug(dumpHeader(r))

// 	fmt.Fprintf(w, "wip")
// 	return
// })

// var supportedChannels = map[string]bool{
// 	"dev":    true,
// 	"beta":   true,
// 	"stable": true,
// }
var Updater = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/updater" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if r.Method == "POST" {
		if len(r.URL.Query()["channel"]) != 1 {
			http.Error(w, "missing: channel", http.StatusBadRequest)
			return
		}
		channel := r.URL.Query()["channel"][0]
		_, ok := cfg.SupportedChannels[channel]
		if !ok {
			Logger.Error("bad channel: " + channel)
			http.Error(w, "bad channel: "+channel, http.StatusBadRequest)
			return
		}
		cfg.TurkeyUpdater.Start(channel)
		Set_listeningChannelLabel(channel) //persist to k8s-deployment-label to recover across pod reboot

		w.WriteHeader(200)
		return
	}
	if r.Method == "GET" {
		fmt.Fprint(w, cfg.TurkeyUpdater.Channel(), " --> ", cfg.TurkeyUpdater.Containers())
		return
	}
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)

})
