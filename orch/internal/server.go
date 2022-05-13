package internal

import (
	"context"
	"crypto/tls"
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

	Logger.Debug("Server is starting...")

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
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		},
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		Logger.Debug("Server is shutting down...")
		atomic.StoreInt32(&Healthy, 0)

		ctx, cancel := context.WithTimeout(context.Background(),
			1*time.Second) //default = 30 or longer probably i guess depends on the thing
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			Logger.Sugar().Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()
	Logger.Debug("Server is ready to handle requests at: " + listenAddr)
	atomic.StoreInt32(&Healthy, 1)

	if err := server.ListenAndServeTLS("cert.pem", "key.pem"); err != nil && err != http.ErrServerClosed {
		Logger.Sugar().Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}
	<-done
	Logger.Debug("Server stopped")
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
				Logger.Debug("(" + requestID + "):" + r.Method + "@" + r.URL.Path + " <- " + r.RemoteAddr)
			}()
			next.ServeHTTP(w, r)
		})
	}
}
