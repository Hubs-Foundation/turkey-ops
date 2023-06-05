package internal

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

var Root_Pausing = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	// http.Error(w, "this hubs' paused, click the duck to try to unpause it", http.StatusOK)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	bytes, err := ioutil.ReadFile("./_statics/pausing.html")
	if err != nil {
		Logger.Sugar().Errorf("%v", err)
	}
	fmt.Fprint(w, string(bytes))

})
