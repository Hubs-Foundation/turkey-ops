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
	fmt.Fprintf(w, html)
	return
})

var Global_404_launch_fallback = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/global_404_fallback" || r.Method != "GET" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	internal.GetLogger().Debug(dumpHeader(r))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `
	<h1> your hubs infra's starting up, when the code's there, it's not yet </h1>
	this is still wip ... <b>/Global_404_launch_fallback</b> ... <br>
	todo: <br>
	1. check for (free) subdomain <br>
	2. scale it back up <br>
	3. need a better looking page here
	`
	fmt.Fprintf(w, html)
	return
})
