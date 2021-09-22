package handlers

import (
	"encoding/json"
	"main/utils"
	"net/http"
	"sync/atomic"
)

func dumpHeader(r *http.Request) string {
	headerBytes, _ := json.Marshal(r.Header)
	return string(headerBytes)
}

var KeepAlive = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

})

// var Dummy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

// 	fmt.Println(" ~~~ hello from /Dummy ~~~ ~~~ ~~~ dumping r !!!")

// 	headerBytes, _ := json.Marshal(r.Header)

// 	fmt.Println(string(headerBytes))

// 	cookieMap := make(map[string]string)
// 	for _, c := range r.Cookies() {
// 		cookieMap[c.Name] = c.Value
// 	}
// 	cookieJson, _ := json.Marshal(cookieMap)
// 	fmt.Println(string(cookieJson))

// 	fmt.Println(" ~~~ /Dummy ~~~ ~~~ ~~~ done !!!")

// 	os.Exit(1)

// })
func Healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&utils.Healthy) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}
