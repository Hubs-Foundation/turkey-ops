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

	// pwd, _ := os.Getwd()
	router := http.NewServeMux()
	router.Handle("/_statics/", http.StripPrefix("/_statics/", http.FileServer(http.Dir("_statics"))))
	router.Handle("/console", handlers.Console)
	router.Handle("/hc_deploy", privateEndpoint("placeholder")(handlers.Hc_deploy))
	router.Handle("/hc_get", handlers.Hc_get)
	router.Handle("/LogStream", handlers.LogStream)
	router.Handle("/Healthz", handlers.Healthz())

	router.Handle("/hc_delNS", handlers.Hc_delNS)
	router.Handle("/hc_delDB", handlers.Hc_delDB)

	// router.Handle("/KeepAlive", handlers.KeepAlive)
	// router.Handle("/Dummy", handlers.Dummy)
	internal.StartServer(router, 888)

}

func privateEndpoint(requiredRole string) func(http.Handler) http.Handler {
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
