package internal

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"text/template"
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

func Dummy() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "nope")

	})
}

type hasCfg struct {
	Tier string
	// Thumbnail_server       string
	// Base_assets_path       string
	// Non_cors_proxy_domains string
	// Reticulum_server       string
	// Cors_proxy_server      string
	// Shortlink_domain       string
	// Hubs_server            string
	// Sentry_dsn             string
	// Is_moz                 string
	Subdomain string
	Domain    string
	HubDomain string
}

func Has() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		subdomain := getSubDomain(r.Host)
		fileName := "_" + r.URL.Path

		Logger.Sugar().Debugf("subdomain: %s, fileName: %s", subdomain, fileName)

		// Parse and execute the template file
		tmpl, err := template.ParseFiles(fileName)
		if err != nil {
			http.Error(w, "", 404)
			return
		}

		tier := "p0"

		// todo -- track subdomains and tiers so we can use this to other tiers, handle chachings etc. to avoid blow up k8s master apis

		err = tmpl.Execute(
			w,
			hasCfg{
				Tier:      tier,
				Subdomain: subdomain,
				Domain:    Cfg.Domain,
				HubDomain: Cfg.HubDomain,
				// Thumbnail_server:       "nearspark.reticulum.io",
				// Base_assets_path:       "https://" + subdomain + ".assets." + Cfg.Domain + "/hubs/",
				// Non_cors_proxy_domains: subdomain + "." + Cfg.Domain + "," + subdomain + ".assets." + Cfg.Domain,
				// Reticulum_server:       subdomain + "." + Cfg.HubDomain,
				// Cors_proxy_server:      "hubs-proxy.com",
				// Shortlink_domain:       subdomain + "." + Cfg.HubDomain,
				// Hubs_server:            subdomain + "." + Cfg.HubDomain,
				// Sentry_dsn:             "foobar",
				// Is_moz:                 "false",
			})
		if err != nil {
			http.Error(w, "", 403)
		}
	})
}

// func NBSRV() http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Fprintf(w, `
// <!DOCTYPE html>
// <html>
// <head>
// 	<style>
// 		body {
// 			background-color: black;
// 			color: white;
// 			display: flex;
// 			justify-content: center;
// 			align-items: center;
// 			height: 100vh;
// 			flex-direction: column;
// 		}
// 		#pic {
// 			width: 200px;
// 			height: 200px;
// 		}
// 	</style>
// </head>
// <body>
// 	<img id="duckPic" src="" />
// 	<div id="msg">waiting for backend, please try again later</div>

// 	<script>

// 	</script>
// </body>
// </html>
// 		`)
// 	})
// }
