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
	//public
	router.Handle("/login", internal.Login())
	router.Handle("/_oauth", internal.Oauth())
	router.Handle("/_fxa", internal.Oauth())
	router.Handle("/logout", internal.Logout())
	router.Handle("/authn", internal.Authn())
	router.Handle("/chk_cookie", internal.ChkCookie())
	router.Handle("/chk_token", internal.ChkToken())

	//private
	router.Handle("/_healthz", internal.Healthz())
	//get: curl localhost:9001/zaplvl
	//set: curl -X PUT -d 'level=debug' localhost:9001/zaplvl
	router.Handle("/zaplvl", internalEndpoint()(internal.Atom))
	router.Handle("/gimmie_test_jwt_cookie", internalEndpoint()(internal.GimmieTestJwtCookie()))
	router.Handle("/gimmie_pubkey", internalEndpoint()(internal.Gimmie_pubkey()))
	router.Handle("/turkeyauthproxy", internal.AuthnProxy())
	router.Handle("/", internal.AuthnProxy())

	//start http server
	go internal.StartNewServer(router, 9001, false)
	//start https server
	internal.StartNewServer(router, 9002, true)
}

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
