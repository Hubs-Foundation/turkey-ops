package main

import (
	"net/http"
	"strconv"

	"main/internal"
	"main/internal/handlers"
)

func main() {
	internal.InitLogger()
	internal.MakeCfg()
	internal.MakePgxPool()

	//++++++++++++++++++++++++ testing...
	internal.StartLruCache()
	internal.Cfg.K8ss_local.StartWatching_HcNs()
	//-----------------------------------

	router := http.NewServeMux()

	router.Handle("/Healthz", handlers.Healthz())

	router.Handle("/console", requireRole("foobar")(handlers.Console))

	router.Handle("/_statics/", http.StripPrefix("/_statics/", http.FileServer(http.Dir("_statics"))))
	router.Handle("/LogStream", handlers.LogStream)

	router.Handle("/hc_get", handlers.Hc_get)
	router.Handle("/hc_deploy", requireRole("foobar")(handlers.Hc_deploy))

	router.Handle("/hc_del", handlers.Hc_del)

	// router.Handle("/admin-info", handlers.Ita_admin_info)
	// router.Handle("/configs/reticulum/ps", handlers.Ita_cfg_ret_ps)
	router.Handle("/hc_launch_fallback", handlers.HC_launch_fallback)
	router.Handle("/global_404_fallback", handlers.Global_404_launch_fallback)

	router.Handle("/webhooks/dockerhub", handlers.Dockerhub)
	router.Handle("/webhooks/ghaturkey", handlers.GhaTurkey)

	router.Handle("/ytdl/api/info", handlers.Ytdl)

	router.Handle("/tco_aws", handlers.TurkeyAws)
	router.Handle("/tco_gcp", handlers.TurkeyGcp)
	router.Handle("/tco_gcp_del", handlers.TurkeyGcp_del)

	router.Handle("/Dummy", handlers.Dummy)

	port, err := strconv.Atoi(internal.Cfg.Port)
	if err != nil {
		internal.GetLogger().Panic("bad port: " + err.Error())
	}
	internal.StartServer(router, port)

}

//todo: make a real rbac -- this just checks if the user's email got an @mozilla.com at the end
func requireRole(role string) func(http.Handler) http.Handler {
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
