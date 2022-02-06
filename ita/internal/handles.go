package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
var Tu_channel = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// if r.URL.Path != "/tu_channel" {
	// 	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	// 	return
	// }
	if r.Method != "POST" && r.URL.Path == "/tu_channel" && len(r.URL.Query()["channel"]) == 1 {
		channel := r.URL.Query()["channel"][0]
		_, ok := cfg.SupportedChannels[channel]
		if !ok {
			Logger.Error("bad channel: " + channel)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		cfg.ListeningChannel = channel
		ns, err := cfg.K8sClientSet.CoreV1().Namespaces().Get(context.Background(), cfg.PodNS, metav1.GetOptions{})
		if err != nil {
			Logger.Error("failed to get local NS: " + cfg.PodNS)
		}
		ns.Labels["CHANNEL"] = channel
		_, err = cfg.K8sClientSet.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
		if err != nil {
			Logger.Error("failed to update ns.labels with new channel value: " + channel)
		}

		//change of container.spec.env.CHANNEL should reboot the pod anyway but just in case?
		cfg.TurkeyUpdater = NewTurkeyUpdater()
		_, err = cfg.TurkeyUpdater.Start()
		if err != nil {
			Logger.Error(err.Error())
		}
	}
	if r.Method != "GET" && r.URL.Path == "/tu_channel" {
		fmt.Fprint(w, cfg.ListeningChannel)
		return
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)

})