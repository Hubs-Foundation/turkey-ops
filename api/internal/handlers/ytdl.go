package handlers

import (
	"main/internal"
	"net/http"
)

var Ytdl = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/info" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	internal.GetLogger().Debug(dumpHeader(r))

	http.Error(w, "comming soon", http.StatusNotImplemented)
	return
})
