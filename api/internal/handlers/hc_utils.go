package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"main/internal"
	"net/http"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

})

var Ita_cfg_ret_ps = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

})

var HC_launch_fallback = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hc_launch_fallback" || r.Method != "GET" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	internal.GetLogger().Debug(dumpHeader(r))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `
	<h1> your hubs services are warming up, <br>
	it went cold because it's a free tier<br>
	or the pod somehow dead... <br>
	anyway check back in 30-ish-seconds </h1> <br>
	this is still wip ... <b>/hc_launch_fallback</> ... <br>
	todo: <br>
	need a better looking page here
	`
	fmt.Fprint(w, html)

})

// type g404NsFindings struct{
// 	lastchecked time.Time
// 	goodNs bool
// }
// var g404cache = map[string]g404NsFindings{}

var g404RespMsg = `
<h1> your hubs infra's starting up </h1>
this is still wip ... <b>/Global_404_launch_fallback</b> ... <br>
todo: <br>
1. check for (free) subdomain <br>
2. scale it back up <br>
3. need a better looking page here
`

// todo: put strict rate limit on this endpoint and add caching to deflect/protect against ddos
var Global_404_launch_fallback = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/global_404_fallback" || r.Method != "GET" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	nsName := "hc-" + strings.Split(r.Header.Get("X-Forwarded-Host"), ".")[0]

	///////////////////////////////////
	// internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces()
	///////////////////////////////////

	//todo: implement some sort of caching mechanism here to avoid bugging k8s master every time ... best without redis, if possible
	ns, err := internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().Get(context.Background(), nsName, v1.GetOptions{})
	if err != nil || ns == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	go wakeupHcNs(nsName)

	internal.GetLogger().Debug(dumpHeader(r))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, g404RespMsg)

})

func wakeupHcNs(ns string) {

	//todo: get and handle tier configs

	//scale things back up in this namespace
	ds, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).List(context.Background(), v1.ListOptions{})
	if err != nil {
		internal.GetLogger().Error("wakeupHcNs - failed to list deployments: " + err.Error())
	}

	scaleUpTo := 1
	for _, d := range ds.Items {
		d.Spec.Replicas = pointerOfInt32(scaleUpTo)
		_, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).Update(context.Background(), &d, v1.UpdateOptions{})
		if err != nil {
			internal.GetLogger().Error("wakeupHcNs -- failed to scale <ns: " + ns + ", deployment: " + d.Name + "> back up: " + err.Error())
		}
	}

}

func pointerOfInt32(i int) *int32 {
	int32i := int32(i)
	return &int32i
}
