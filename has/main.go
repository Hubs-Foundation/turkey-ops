package main

import (
	"main/internal"
	"net/http"
)

func main() {
	internal.InitLogger()
	router := http.NewServeMux()
	router.Handle("/dummy", internal.Dummy())
	router.Handle("/hubs", internal.Has())
	router.Handle("/spoke", internal.Has())

	internal.StartNewServer(router, 9001, false)

}
