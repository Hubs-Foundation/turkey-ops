package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"main/internal"
	"net/http"
	"strings"
	"time"

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

	internal.Logger.Sugar().Debugf("dump headers: %v", r.Header)

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

var g404_std_RespMsg = `
<h1> your hubs infra's starting up </h1>
disclaimer: this page is still wip ... <b>/Global_404_launch_fallback</b> ... <br>
todo(internal only): <br>
1. a better looking page here <br>
`
var g404_err_RespMsg = `
<h1> your hubs infra's dead ... <br> but don't worry because some engineers on our end's getting a pagerduty for it </h1>
disclaimer: this page is still wip ... <b>/Global_404_launch_fallback</b> ... <br>
todo(internal only): <br>
1. a better looking page here <br>
`

// todo: put strict rate limit on this endpoint and add caching to deflect/protect against ddos
var Global_404_launch_fallback = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/global_404_fallback" || r.Method != "GET" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	internal.Logger.Sugar().Debugf("dump headers: %v", r.Header)

	nsName := "hc-" + strings.Split(r.Header.Get("X-Forwarded-Host"), ".")[0]

	// not requesting a hubs cloud namespace == bounce
	if !internal.HC_NS_TABLE.Has(nsName) {
		internal.Logger.Debug("404 bounc / !internal.HC_NS_TABLE.Has for: " + nsName)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	notes := internal.HC_NS_TABLE.Get(nsName)

	// high frequency pokes == bounce
	coolDown := 15 * time.Minute
	if time.Since(notes.Lastchecked) < coolDown {
		internal.Logger.Debug("on coolDown bounc for: " + nsName)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, g404_std_RespMsg)
		return
	}

	//todo: check if Labeled with status=paused, otherwise it's probably an error/exception because the request should be catched by higher priority ingress rules inside hc-namespace
	//todo: check tiers for scaling configs
	//todo: test HPA (horizontal pod autoscaler)'s min settings instead of

	//just scale it back up to 1 for now

	go wakeupHcNs(nsName)
	internal.HC_NS_TABLE.Set(nsName, internal.HcNsNotes{Lastchecked: time.Now()})

	internal.Logger.Debug("wakeupHcNs launched for nsName: " + nsName)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, g404_std_RespMsg)

})

func wakeupHcNs(ns string) {

	//todo: get and handle tier configs

	//scale things back up in this namespace
	ds, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).List(context.Background(), v1.ListOptions{})
	if err != nil {
		internal.Logger.Error("wakeupHcNs - failed to list deployments: " + err.Error())
	}

	scaleUpTo := 1
	for _, d := range ds.Items {
		d.Spec.Replicas = pointerOfInt32(scaleUpTo)
		_, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).Update(context.Background(), &d, v1.UpdateOptions{})
		if err != nil {
			internal.Logger.Error("wakeupHcNs -- failed to scale <ns: " + ns + ", deployment: " + d.Name + "> back up: " + err.Error())
		}
	}

}

func pointerOfInt32(i int) *int32 {
	int32i := int32(i)
	return &int32i
}
