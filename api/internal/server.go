package internal

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// type key int
// const requestIDKey key = 0
const requestIDKey int = 0

var listenAddr string
var Healthy int32

func StartServer(router *http.ServeMux, port int) {
	flag.StringVar(&listenAddr, "listen-addr", ":"+strconv.Itoa(port), "server listen address")
	flag.Parse()

	Logger.Info("Server is starting...")

	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      tracing(nextRequestID)(logging()(router)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		Logger.Info("Server is shutting down...")
		atomic.StoreInt32(&Healthy, 0)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			Logger.Sugar().Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()
	Logger.Info("Server is ready to handle requests at" + listenAddr)
	atomic.StoreInt32(&Healthy, 1)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		Logger.Sugar().Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}
	<-done
	Logger.Info("Server stopped")
}

func logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				Logger.Debug("new request,", zap.String("id", requestID),
					zap.String("method", r.Method), zap.String("path", r.URL.Path),
					zap.String("RemoteAddr", r.RemoteAddr), zap.String("UserAgent", r.UserAgent()))
			}()
			next.ServeHTTP(w, r)
		})
	}
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
