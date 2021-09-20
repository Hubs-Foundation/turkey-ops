package main

import (
	"main/internal"
	"net/http"
)

var (
	cfg *internal.Config
)

func main() {

	internal.MakeCfg()
	internal.InitLogger()

	router := http.NewServeMux()
	router.Handle("/healthz", internal.Healthz())
	router.Handle("/login", internal.Login())
	router.Handle("/logout", internal.Logout())
	router.Handle("/traefik-ip", internal.TraefikIp())
	router.Handle("/_oauth", internal.Oauth())

	internal.StartServer(router, 9001)
}
