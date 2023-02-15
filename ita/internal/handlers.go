package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
		"server_domain":        cfg.HubDomain,
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
		if len(r.URL.Query()["channel"]) != 1 || r.URL.Query()["channel"][0] == "" {
			http.Error(w, "missing: channel", http.StatusBadRequest)
			return
		}
		channel := r.URL.Query()["channel"][0]
		// _, ok := cfg.SupportedChannels[channel]
		// if !ok {
		// 	Logger.Error("bad channel: " + channel)
		// 	http.Error(w, "bad channel: "+channel, http.StatusBadRequest)
		// 	return
		// }

		Deployment_setLabel("CHANNEL", channel)
		cfg.TurkeyUpdater.Start()

		w.WriteHeader(200)
		return
	}
	if r.Method == "GET" {
		fmt.Fprint(w, cfg.TurkeyUpdater.Channel(), " --> ", cfg.TurkeyUpdater.Containers())
		return
	}
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)

})

func HubInfraStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		deployments, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error: " + err.Error()})
			return
		}
		for _, d := range deployments.Items {
			if k8s_isDeploymentRunning(&d) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{"status": "deploying"})
				return
			}
		}

		pods, err := cfg.K8sClientSet.CoreV1().Pods(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error: " + err.Error()})
			return
		}
		if err := k8s_waitForPods(pods, 1*time.Second); err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "podsPending"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ready"})
		return
	})
}

var ClusterIps = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/z/meta/cluster-ips" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		res := StreamNodes
		json.NewEncoder(w).Encode(res)
		return
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)

})
var ClusterIpsList = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/z/meta/cluster-ips/list" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		res := StreamNodeIpList
		fmt.Fprint(w, res)
		return
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)

})

const MAX_UPLOAD_SIZE = 1024 * 1024 * 1024 // 1GB
// Progress is used to track the progress of a file upload.It implements the io.Writer interface so it can be passed to an io.TeeReader()
type Progress struct {
	TotalSize int64
	BytesRead int64
}

// Write is used to satisfy the io.Writer interface. Instead of writing somewhere, it simply aggregates the total bytes on each read
func (pg *Progress) Write(p []byte) (n int, err error) {
	n, err = len(p), nil
	pg.BytesRead += int64(n)
	pg.Print()
	return
}

// Print displays the current progress of the file upload
func (pg *Progress) Print() {
	if pg.BytesRead == pg.TotalSize {
		Logger.Sugar().Debugf("DONE! %v", pg)
		return
	}

	fmt.Printf("File upload in progress: %d\n", pg.BytesRead)
}

var Handle_NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
})
