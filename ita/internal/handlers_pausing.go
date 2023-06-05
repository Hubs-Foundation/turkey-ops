package internal

import "net/http"

var Root_Pausing = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	http.Error(w, "this hubs' paused, click the duck to try to unpause it", http.StatusOK)
})
