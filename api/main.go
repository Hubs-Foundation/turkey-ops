package main

import (
	"net/http"

	"main/internal"
	"main/internal/handlers"
)

func main() {

	internal.InitLogger()
	internal.MakeCfg()
	internal.MakePgxPool()

	router := http.NewServeMux()

	router.Handle("/Healthz", handlers.Healthz())

	router.Handle("/console", requireRole("foobar")(handlers.Console))

	router.Handle("/_statics/", http.StripPrefix("/_statics/", http.FileServer(http.Dir("_statics"))))
	router.Handle("/LogStream", handlers.LogStream)

	router.Handle("/hc_get", handlers.Hc_get)
	router.Handle("/hc_deploy", requireRole("foobar")(handlers.Hc_deploy))

	router.Handle("/hc_delNS", handlers.Hc_delNS)
	router.Handle("/hc_delDB", handlers.Hc_delDB)

	router.Handle("/admin-info", handlers.Ita_admin_info)
	router.Handle("/configs/reticulum/ps", handlers.Ita_cfg_ret_ps)

	router.Handle("/Dummy", handlers.Dummy)
	internal.StartServer(router, 888)

}

//todo: make a real one -- this just checks if the user's email got an @mozilla.com at the end
func requireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email := r.Header.Get("X-Forwarded-UserEmail")
			if email[len(email)-12:] != "@mozilla.com" {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

//todo: this only sort-of works, make it really works
func localOnly(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Forwarded-For") != "" {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
