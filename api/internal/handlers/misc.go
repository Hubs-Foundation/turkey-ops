package handlers

import (
	"main/internal"
	"net/http"
	"sync/atomic"
)

// var KeepAlive = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

// })

var Dummy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	// conn, err := internal.PgxPool.Acquire(context.Background())
	// if err != nil {
	// 	fmt.Fprintln(os.Stderr, "Error acquiring connection:", err)
	// 	os.Exit(1)
	// }

	// dbName := "ret_geng_test_3"
	// _, err = conn.Exec(context.Background(), "create database "+dbName)
	// if err != nil {
	// 	panic(err)
	// }
	// retSchemaBytes, err := ioutil.ReadFile("./_files/pgSchema.sql")
	// if err != nil {
	// 	panic(err)
	// }
	// dbconn, err := pgx.Connect(context.Background(), internal.Cfg.DBconn+"/"+dbName)
	// if err != nil {
	// 	panic(err)
	// }
	// _, err = dbconn.Exec(context.Background(), string(retSchemaBytes))
	// if err != nil {
	// 	panic(err)
	// }
	// dbconn.Close(context.Background())

	// fmt.Println(" ~~~ hello from /Dummy ~~~ ~~~ ~~~ dumping r !!!")

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
