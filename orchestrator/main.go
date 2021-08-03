package main

import (
	"net/http"

	"main/handlers"
	"main/utils"
)

func main() {

	// pwd, _ := os.Getwd()
	router := http.NewServeMux()

	router.Handle("/_files/", http.StripPrefix("/_files/", http.FileServer(http.Dir("_files"))))

	router.Handle("/", handlers.Root)
	router.Handle("/TurkeyDeployAWS", handlers.TurkeyDeployAWS)
	router.Handle("/LogStream", handlers.LogStream)

	router.Handle("/KeepAlive", handlers.KeepAlive)

	router.Handle("/Dummy", handlers.Dummy)

	utils.StartServer(router, 888)

}
