package internal

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"time"
)

// func Start(router *http.ServeMux, port int) {
// 	startServer(router, port)
// }

type key int

const (
	requestIDKey key = 0
)

var (
	listenAddr string
	Healthy    int32
)

func StartServer(router *http.ServeMux, port int) {
	flag.StringVar(&listenAddr, "listen-addr", ":"+strconv.Itoa(port), "server listen address")
	flag.Parse()

	Logger.Println("Server is starting...")

	nextRequestID := func() string {
		// return fmt.Sprintf("%d", time.Now().UnixNano())
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(time.Now().UnixNano()))
		return base64.RawStdEncoding.EncodeToString(b)
	}

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      tracing(nextRequestID)(logging(Logger)(router)),
		ErrorLog:     Logger,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0 * time.Second,
		IdleTimeout:  3600 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		Logger.Println("Server is shutting down...")
		atomic.StoreInt32(&Healthy, 0)

		ctx, cancel := context.WithTimeout(context.Background(),
			1*time.Second) //default = 30 or longer probably i guess depends on the thing
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			Logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()
	Logger.Println("Server is ready to handle requests at", listenAddr)
	atomic.StoreInt32(&Healthy, 1)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		Logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}
	<-done
	Logger.Println("Server stopped")
}
func tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
func logging(Logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				// Logger.Println(requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
				Logger.Println("(" + requestID + "):" + r.Method + "@" + r.URL.Path + " <- " + r.RemoteAddr)
			}()
			next.ServeHTTP(w, r)
		})
	}
}
