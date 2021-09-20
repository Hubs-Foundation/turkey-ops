package main

import (
	"net/http"

	"main/handlers"
	"main/utils"
)

func main() {

	// pwd, _ := os.Getwd()
	router := http.NewServeMux()

	router.Handle("/_statics/", http.StripPrefix("/_statics/", http.FileServer(http.Dir("_statics"))))

	router.Handle("/", handlers.Console)
	router.Handle("/orchestrator", handlers.Orchestrator)
	router.Handle("/LogStream", handlers.LogStream)
	router.Handle("/KeepAlive", handlers.KeepAlive)
	router.Handle("/Dummy", handlers.Dummy)
	utils.StartServer(router, 888)

}
