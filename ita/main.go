package main

import (
	"fmt"
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
	//public endpoints
	router.Handle("/z/meta/cluster-ips", internal.ClusterIps)
	router.Handle("/z/meta/cluster-ips/list", internal.ClusterIpsList)
	//turkeyUpdater endpoints
	router.Handle("/updater", internal.Updater)
	//utility endpoints
	router.Handle("/zaplvl", privateEndpoint("dev")(internal.Atom))
	router.Handle("/healthz", internal.Healthz())
	router.Handle("/hub_status", internal.HubInfraStatus())
	//turkeyauth protected public endpoints
	router.Handle("/upload", internal.Upload)
	router.Handle("/deploy/hubs", internal.DeployHubs)

	go internal.StartNewServer(router, 6000, false)
	internal.StartNewServer(router, 6001, true)

}

func privateEndpoint(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("~~~~~~~~~~~privateEndpoint~~~~~~~~~~~")
			next.ServeHTTP(w, r)
		})
	}
}
