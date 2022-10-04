package main

import (
	"fmt"
	"main/internal"
	"net/http"
	"strings"
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
	cron_3m := internal.NewCron("cron_3m", 3*time.Minute)
	if strings.HasPrefix(internal.GetCfg().PodNS, "hc-") {
		cron_1m.Load("pauseJob", internal.Cronjob_pauseHC)
	}
	if internal.GetCfg().PodNS == "turkey-services" {
		cron_3m.Load("cleanupFailedPods", internal.Cronjob_cleanupFailedPods)
		cron_3m.Load("SurveyStreamNodes", internal.Cronjob_SurveyStreamNodes)
		cron_3m.Start()
		cron_1m.Load("turkeyBuildPublisher", internal.Cronjob_publishTurkeyBuildReport)
		cron_1m.Load("HcHealthchecks", internal.Cronjob_HcHealthchecks)
		cron_1m.Start()
	}
	//#############################################
	//################# server ####################
	//#############################################

	router := http.NewServeMux()
	//legacy ita endpoints
	router.Handle("/admin-info", internal.Ita_admin_info)
	router.Handle("/configs/reticulum/ps", internal.Ita_cfg_ret_ps)
	//turkeyUpdater endpoints
	router.Handle("/updater", internal.Updater)
	//utility endpoints
	router.Handle("/zaplvl", privateEndpoint("dev")(internal.Atom))
	router.Handle("/healthz", internal.Healthz())
	router.Handle("/hub_status", internal.HubInfraStatus())
	//public endpoints
	router.Handle("/meta/cluster-ips", internal.ClusterIps)

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
