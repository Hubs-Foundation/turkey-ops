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
	internal.InitProxyman()

	router := http.NewServeMux()
	router.Handle("/", internal.AuthnProxy())

	router.Handle("/healthz", internal.Healthz())
	router.Handle("/login", internal.Login())
	router.Handle("/_oauth", internal.Oauth())
	router.Handle("/_fxa", internal.Oauth())

	router.Handle("/logout", internal.Logout())
	// router.Handle("/traefik-ip", internal.TraefikIp())
	router.Handle("/authn", internal.Authn())

	router.Handle("/chk_cookie", internal.ChkCookie())

	router.Handle("/gimmie_test_jwt_cookie", internalEndpoint()(internal.GimmieTestJwtCookie()))

	router.Handle("/zaplvl", internalEndpoint()(internal.Atom))
	//curl localhost:9001/zaplvl
	//apk add curl; curl -X PUT -d 'level=debug' localhost:9001/zaplvl

	internal.StartServer(router, 9001)
}

// func privateEndpoint(requiredRole string) func(http.Handler) http.Handler {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			fmt.Println("~~~~~~~~~~~privateEndpoint~~~~~~~~~~~")
// 			next.ServeHTTP(w, r)
// 		})
// 	}
// }
func internalEndpoint() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Forwarded-For") != "" {
				http.NotFound(w, r)
				return
			}
			fmt.Println("~~~~~~~~~~~internalEndpoint~~~~~~~~~~~")
			next.ServeHTTP(w, r)
		})
	}
}
