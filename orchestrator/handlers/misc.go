package handlers

import (
	"fmt"
	"net/http"
	"os"
)

var KeepAlive = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

})

var Dummy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	fmt.Println(" ~~~ hello ~~~ ")

	// turkeyUtils.MakeKeys("dummyTest-name", "./tmpKeys")

	os.Exit(1)

})
