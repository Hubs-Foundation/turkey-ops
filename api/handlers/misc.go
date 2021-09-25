package handlers

import (
	"net/http"
	"sync/atomic"

	"main/internal"
)

// var KeepAlive = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

// })

var Dummy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	// conn, err := internal.PgxPool.Acquire(context.Background())
	// if err != nil {
	// 	fmt.Fprintln(os.Stderr, "Error acquiring connection:", err)
	// 	os.Exit(1)
	// }

	// dbName := "geng_test_1"
	// _, err = conn.Exec(context.Background(), "create database "+dbName)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(" ~~~ hello from /Dummy ~~~ ~~~ ~~~ dumping r !!!")

	// headerBytes, _ := json.Marshal(r.Header)

	// fmt.Println(string(headerBytes))

	// cookieMap := make(map[string]string)
	// for _, c := range r.Cookies() {
	// 	cookieMap[c.Name] = c.Value
	// }
	// cookieJson, _ := json.Marshal(cookieMap)
	// fmt.Println(string(cookieJson))

	// fmt.Println(" ~~~ /Dummy ~~~ ~~~ ~~~ done !!!")

	// os.Exit(1)

})

func Healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&internal.Healthy) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}
