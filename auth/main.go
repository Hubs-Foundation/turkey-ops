package main

import (
	"fmt"
	"main/internal"
	"net/http"
)

var (
	cfg *internal.Config
)

func main() {
	internal.InitLogger()
	internal.MakeCfg()

	router := http.NewServeMux()
	router.Handle("/healthz", internal.Healthz())
	router.Handle("/login", internal.Login())
	router.Handle("/_oauth", internal.Oauth())
	router.Handle("/logout", internal.Logout())
	// router.Handle("/traefik-ip", internal.TraefikIp())
	router.Handle("/authn", internal.Authn())

	router.Handle("/zaplvl", privateEndpoint("dev")(internal.Atom))

	internal.StartServer(router, 9001)
}

func privateEndpoint(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("~~~~~~~~~~~privateEndpoint~~~~~~~~~~~")
			// requestID := r.Header.Get("X-Request-Id")
			// if requestID == "" {
			// 	requestID = nextRequestID()
			// }
			// ctx := context.WithValue(r.Context(), reqIdKey, requestID)
			// w.Header().Set("X-Request-Id", requestID)

			next.ServeHTTP(w, r)
		})
	}
}
