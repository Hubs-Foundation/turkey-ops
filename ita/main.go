package main

import (
	"main/internal"
	"net/http"
	"time"
)

var ()

func main() {

	internal.InitLogger()
	internal.MakeCfg()

	//#############################################
	//################# cron jobs #################
	//#############################################
	cron_1m := internal.NewCron("cron_1m", 1*time.Minute)
	cron_15m := internal.NewCron("cron_15m", 15*time.Minute)

	// if strings.HasPrefix(internal.GetCfg().PodNS, "hc-") {
	// 	// cron_1m.Load("pauseJob", internal.Cronjob_pauseHC)
	// 	// cron_1m.Load("HcHealthchecks", internal.Cronjob_HcHealthchecks)
	// }

	if internal.GetCfg().PodNS == "turkey-services" {
		cron_1m.Load("turkeyBuildPublisher", internal.Cronjob_publishTurkeyBuildReport)
		cron_15m.Load("cleanupFailedPods", internal.Cronjob_cleanupFailedPods)
		cron_15m.Load("SurveyStreamNodes", internal.Cronjob_SurveyStreamNodes)
		internal.Cronjob_SurveyStreamNodes(888 * time.Microsecond)
	}
	cron_1m.Start()
	cron_15m.Start()

	//#############################################
	//################# server ####################
	//#############################################

	router := http.NewServeMux()
	//legacy(dummy) ita endpoints
	router.Handle("/admin-info", internal.Ita_admin_info)
	router.Handle("/configs/reticulum/ps", internal.Ita_cfg_ret_ps)
	//public api endpoints
	router.Handle("/z/meta/cluster-ips", internal.ClusterIps)
	router.Handle("/z/meta/cluster-ips/list", internal.ClusterIpsList)

	//turkeyUpdater endpoints
	router.Handle("/updater", internal.Updater)
	//utility endpoints
	// router.Handle("/zaplvl", privateEndpoint("dev")(internal.Atom))
	router.Handle("/zaplvl", internal.Atom)
	router.Handle("/healthz", internal.Healthz())
	router.Handle("/hub_status", internal.HubInfraStatus())
	//private api endpoints
	router.Handle("/upload", internal.Upload)
	router.Handle("/deploy", internal.Deploy)
	router.Handle("/undeploy", internal.Undeploy)
	router.Handle("/custom-domain", internal.CustomDomain)
	router.Handle("/letsencrypt-account-collect", internal.LetsencryptAccountCollect)
	router.Handle("/dump-worklog", internal.DumpWorkLog)

	//turkeyauth protected public api endpoints
	router.Handle("/api/ita/upload", chk_hat_hdr()(internal.Upload))
	router.Handle("/api/ita/deploy", chk_hat_hdr()(internal.Deploy))
	router.Handle("/api/ita/undeploy", chk_hat_hdr()(internal.Undeploy))
	router.Handle("/api/ita/custom-domain", chk_hat_hdr()(internal.CustomDomain))

	go internal.StartNewServer(router, 6000, false)
	internal.StartNewServer(router, 6001, true)

}

//check turkeyauthtoken header
func chk_hat_hdr() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			internal.Logger.Debug("~~~~~~~~~~~chk_TatHdr~~~~~~~~~~~")
			token := r.Header.Get("turkeyauthtoken")
			if token == "" {
				internal.Logger.Debug("reject -- no token")
				// internal.Handle_NotFound(w, r)
				http.NotFound(w, r)
				return
			}
			resp, err := http.Get("http://turkeyauth.turkey-services:9001/chk_token?token=" + token)
			if err != nil {
				internal.Logger.Sugar().Debugf("reject -- err@chk_token: %v", err)
				internal.Handle_NotFound(w, r)
				return
			} else if resp.StatusCode != http.StatusOK {
				internal.Logger.Sugar().Debugf("reject -- bad resp.StatusCode: %v", resp.StatusCode)
				internal.Handle_NotFound(w, r)
				return
			}
			email := resp.Header.Get("verified-UserEmail")
			rootUserEmail := internal.GetCfg().RootUserEmail
			internal.Logger.Sugar().Debugf("verified-UserEmail: %v, rootUserEmail: %v", resp.StatusCode, rootUserEmail)
			if email != rootUserEmail {
				internal.Logger.Sugar().Debugf("reject -- bad verified-UserEmail -- has: %v, need: %v", email, rootUserEmail)
				internal.Handle_NotFound(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
