package handlers

import (
	"fmt"
	"net/http"
)

var Ytdl = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ytdl/api/info" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// internal.GetLogger().Debug(dumpHeader(r))
	fmt.Println("headers: ", dumpHeader(r))
	fmt.Println("params:", r.URL.Query())

	http.Error(w, "comming soon", http.StatusNotImplemented)
	return
})
