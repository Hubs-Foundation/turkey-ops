package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"main/internal"
	"main/internal/handlers"
)

func main() {

	// ############################## inits ##############################
	internal.InitLogger()
	internal.MakeCfg()
	internal.InitSingletons()
	internal.MakeDbs(internal.Cfg.IsRoot)
	internal.TrcCmBook.StartWatching()
	if internal.Cfg.IsRoot && internal.DashboardDb != nil {
		cron_syncDashboardDb := internal.NewCron("cron_syncDashboardDb", 2*time.Minute)
		cron_syncDashboardDb.Load("Cronjob_syncDashboardDb", handlers.Cronjob_syncDashboardDb)
		cron_syncDashboardDb.Start()
	}

	if internal.Cfg.IsRoot { // root cluster manages peers
		internal.Cfg.PeerMan = internal.NewPeerMan()
	} else { // peer cluster report to root cluster
		// report once right now
		internal.Cronjob_CountHC_phonehome(time.Second)
		// then report every 15 min
		cron_countHC := internal.NewCron("Cronjob_CountHC_phonehome", 15*time.Minute)
		cron_countHC.Load("Cronjob_CountHC_phonehome", internal.Cronjob_CountHC_phonehome)
		cron_countHC.Start()
	}

	cron_15m := internal.NewCron("cron_15m", 15*time.Minute)
	cron_15m.Load("Cronjob_trcCacheBookSurveyor", handlers.Cronjob_trcCacheBookSurveyor)
	cron_15m.Start()

	// ############################## gcp-pubsub (deprecated) ##############################
	// handlers.HandleTurkeyJobs()

	// ############################## routes ##############################

	// pvtEpEnforcer := internal.NewPvtEpEnforcer(
	// 	[]string{
	// 		"turkeydashboard.turkey-services",
	// 		"turkeyauth.turkey-services",
	// 	},
	// )

	// if internal.Cfg.K8ss_local != nil {
	// 	// internal.StartLruCache()
	// 	// internal.Cfg.K8ss_local.StartWatching_HcNs()

	// 	pvtEpEnforcer.StartWatching()
	// }

	router := http.NewServeMux()
	//public endpoints
	router.Handle("/webhooks/dockerhub", handlers.Webhook_dockerhub)
	router.Handle("/webhooks/turkeyjobs", handlers.Webhook_turkeyJobs)
	router.Handle("/webhooks/peerreport", handlers.Webhook_peerReport)
	router.Handle("/webhooks/remote_hc_instance", handlers.Webhook_remote_hc_instance)

	//private endpoints
	router.Handle("/_healthz", handlers.Healthz())
	router.Handle("/console", handlers.Console)
	router.Handle("/_statics/", http.StripPrefix("/_statics/", http.FileServer(http.Dir("_statics"))))
	router.Handle("/LogStream", handlers.LogStream)
	router.Handle("/dump_peerMap", handlers.Dump_peerMap)

	// router.Handle("/hc_instance", pvtEpEnforcer.Filter([]string{
	// 	"*",
	// 	// "turkeydashboard.turkey-services",
	// 	// "turkeyauth.turkey-services",
	// })(handlers.HC_instance))

	router.Handle("/hc_instance", handlers.HC_instance)
	router.Handle("/hc_instance/signed_bucket_url", handlers.HC_instance_getSignedBucketUrl)

	router.Handle("/", handlers.TurkeyReturnCenter)
	router.Handle("/turkey-return-center/", handlers.TurkeyReturnCenter)

	router.Handle("/tco_aws", mozOnly()(handlers.TurkeyAws))
	router.Handle("/tco_gcp", mozOnly()(handlers.TurkeyGcp))

	// router.Handle("/snapshot", handlers.HC_snapshot)

	router.Handle("/hub_domain", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, internal.Cfg.HubDomain)
	}))

	router.Handle("/letsencrypt-account-collect", handlers.LetsencryptAccountCollect)
	// router.Handle("/dump_hcnstable", pvtEpEnforcer.Filter([]string{"*"})(handlers.Dump_HcNsTable))

	router.Handle("/api/v1/", handlers.DashboardApi)

	// ############################## serving ##############################
	port, err := strconv.Atoi(internal.Cfg.Port)
	if err != nil {
		internal.GetLogger().Panic("bad port: " + err.Error())
	}
	go internal.StartNewServer(router, port, false)
	internal.StartNewServer(router, port+1, true)

}

// scratchpad

// todo: make a real rbac -- this just checks if the user's email got an @mozilla.com at the end
func mozOnly() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email := r.Header.Get("X-Forwarded-UserEmail")
			internal.GetLogger().Debug("X-Forwarded-UserEmail: " + email)
			if len(email) < 13 || email[len(email)-12:] != "@mozilla.com" {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// func tfa(role string) func(http.Handler) http.Handler {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			authReq, err := http.NewRequest(http.MethodGet, internal.Cfg.AuthProxyUrl, nil)
// 			if err != nil {
// 				internal.GetLogger().Warn("forward auth failed to make NewRequest: " + err.Error())
// 				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 				return
// 			}
// 			authHttpClient := http.Client{
// 				CheckRedirect: func(r *http.Request, via []*http.Request) error {
// 					return http.ErrUseLastResponse
// 				},
// 				Timeout: 30 * time.Second,
// 			}
// 			CopyHeaders(authReq.Header, r.Header)
// 			authResp, err := authHttpClient.Do(authReq)
// 			if err != nil {
// 				internal.GetLogger().Warn("forward auth failed to send: " + err.Error())
// 				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 				return
// 			}
// 			body, readError := io.ReadAll(authResp.Body)
// 			if readError != nil {
// 				internal.GetLogger().Sugar().Debugf("Error reading body %s. Cause: %s", internal.Cfg.AuthProxyUrl, readError)
// 				w.WriteHeader(http.StatusInternalServerError)
// 				return
// 			}
// 			defer authResp.Body.Close()

// 			internal.GetLogger().Sugar().Debugf("authResp.Header: ", authResp.Header)
// 			// Pass the forward response's body and selected headers if it
// 			// didn't return a response within the range of [200, 300).
// 			if authResp.StatusCode < http.StatusOK || authResp.StatusCode >= http.StatusMultipleChoices {
// 				internal.GetLogger().Sugar().Debugf("auth fail -- got code: %v", authResp.StatusCode)
// 				CopyHeaders(w.Header(), authResp.Header)

// 				// Grab the location header, if any.
// 				redirectURL, err := authResp.Location()

// 				if err != nil {
// 					if !errors.Is(err, http.ErrNoLocation) {
// 						internal.GetLogger().Sugar().Debugf("Error reading response location header %s. Cause: %s", internal.Cfg.AuthProxyUrl, err)
// 						w.WriteHeader(http.StatusInternalServerError)
// 						return
// 					}
// 				} else if redirectURL.String() != "" {
// 					w.Header().Set("Location", redirectURL.String())
// 				}
// 				internal.GetLogger().Debug("redirectURL: " + redirectURL.String())
// 				w.WriteHeader(authResp.StatusCode)
// 				if _, err = w.Write(body); err != nil {
// 					internal.GetLogger().Error(err.Error())
// 				}
// 				return
// 			}
// 			internal.GetLogger().Sugar().Debugf("authResp.Header: ", authResp.Header)
// 			email := authResp.Header.Get("X-Forwarded-UserEmail")
// 			internal.GetLogger().Debug("X-Forwarded-UserEmail: " + email)
// 			r.Header.Set("X-Forwarded-UserEmail", email)
// 			if len(email) < 13 || email[len(email)-12:] != "@mozilla.com" {
// 				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 				return
// 			}
// 			r.RequestURI = r.URL.RequestURI()
// 			next.ServeHTTP(w, r)
// 		})
// 	}
// }

// // CopyHeaders copies http headers from source to destination, it
// // does not overide, but adds multiple headers
// func CopyHeaders(dst http.Header, src http.Header) {
// 	for k, vv := range src {
// 		dst[k] = append(dst[k], vv...)
// 	}
// }

// //probably better to just keep k8s ingress rules explicit
// func localOnly(role string) func(http.Handler) http.Handler {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			if r.Header.Get("X-Forwarded-For") != "" {
// 				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 				return
// 			}
// 			next.ServeHTTP(w, r)
// 		})
// 	}
// }
