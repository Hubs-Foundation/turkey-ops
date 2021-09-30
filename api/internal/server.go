package internal

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"flag"
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

	logger.Debug("Server is starting...")

	nextRequestID := func() string {
		// return fmt.Sprintf("%d", time.Now().UnixNano())
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(time.Now().UnixNano()))
		return base64.RawStdEncoding.EncodeToString(b)
	}

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      tracing(nextRequestID)(logging()(router)),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0 * time.Second,
		IdleTimeout:  3600 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		logger.Debug("Server is shutting down...")
		atomic.StoreInt32(&Healthy, 0)

		ctx, cancel := context.WithTimeout(context.Background(),
			1*time.Second) //default = 30 or longer probably i guess depends on the thing
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			logger.Sugar().Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()
	logger.Debug("Server is ready to handle requests at: " + listenAddr)
	atomic.StoreInt32(&Healthy, 1)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Sugar().Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}
	<-done
	logger.Debug("Server stopped")
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
func logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Debug("(" + requestID + "):" + r.Method + "@" + r.URL.Path + " <- " + r.RemoteAddr)
			}()
			next.ServeHTTP(w, r)
		})
	}
}
