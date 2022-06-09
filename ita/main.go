package main

import (
	"fmt"
	"main/internal"
	"net/http"
)

var ()

func main() {

	internal.InitLogger()
	internal.MakeCfg()

	if internal.GetCfg().Tier == "free" {
		cron_15s := internal.NewCron("pauseJob-15s", "15s")
		cron_15s.Load("pauseJob", internal.Cronjob_pauseJob)
		cron_15s.Start()
	}

	router := http.NewServeMux()
	//legacy ita endpoints
	router.Handle("/admin-info", internal.Ita_admin_info)
	router.Handle("/configs/reticulum/ps", internal.Ita_cfg_ret_ps)
	//turkeyUpdater endpoints
	router.Handle("/updater", internal.Updater)
	//utility endpoints
	router.Handle("/healthz", internal.Healthz())
	router.Handle("/zaplvl", privateEndpoint("dev")(internal.Atom))

	internal.StartServer(router, 6000)

}

func privateEndpoint(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("~~~~~~~~~~~privateEndpoint~~~~~~~~~~~")
			next.ServeHTTP(w, r)
		})
	}
}
