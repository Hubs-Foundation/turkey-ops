package internal

import "net/http"

var Root_Pausing = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	// http.Error(w, "this hubs' paused, click the duck to try to unpause it", http.StatusOK)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, "./_statics/pausing.html")

})
