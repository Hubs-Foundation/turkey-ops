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
	cron := internal.NewCron("dummy-1h", "1h")
	cron.Load("dummy", internal.Cronjob_dummy)

	turkeyUpdater := *internal.NewTurkeyUpdater("dev")
	_, err := turkeyUpdater.Start("turkey-services")
	if err != nil {
		internal.Logger.Error(err.Error())
	}

	router := http.NewServeMux()
	router.Handle("/healthz", internal.Healthz())

	router.Handle("/admin-info", internal.Ita_admin_info)
	router.Handle("/configs/reticulum/ps", internal.Ita_cfg_ret_ps)

	router.Handle("/zaplvl", privateEndpoint("dev")(internal.Atom))

	internal.StartServer(router, 9001)

}

func privateEndpoint(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("~~~~~~~~~~~privateEndpoint~~~~~~~~~~~")
			next.ServeHTTP(w, r)
		})
	}
}
