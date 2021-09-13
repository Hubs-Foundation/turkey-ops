package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"

	"strconv"
)

type key int

const (
	requestIDKey key = 0
)

var (
	listenAddr string
	healthy    int32

	logger = log.New(os.Stdout, "http: ", log.LstdFlags)
	// zonalNameMap = make(map[string]string)
)

func main() {

	router := http.NewServeMux()
	// router.Handle("/", root())
	router.Handle("/healthz", healthz())
	router.Handle("/traefik", traefik())

	startServer(router, 9001)

}

// func root() http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		if r.URL.Path != "/" {
// 			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 			return
// 		}
// 		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
// 		w.Header().Set("X-Content-Type-Options", "nosniff")
// 		w.WriteHeader(http.StatusOK)
// 		fmt.Fprintln(w, "hello "+r.RemoteAddr)
// 	})
// }

func healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&healthy) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}
func traefik() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/traefik" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		r.URL, _ = url.Parse(r.Header.Get("X-Forwarded-Uri"))
		r.Method = r.Header.Get("X-Forwarded-Method")
		r.Host = r.Header.Get("X-Forwarded-Host")
		headerBytes, _ := json.Marshal(r.Header)
		cookieMap := make(map[string]string)
		for _, c := range r.Cookies() {
			cookieMap[c.Name] = c.Value
		}
		cookieJson, _ := json.Marshal(cookieMap)
		fmt.Println("headers: " + string(headerBytes) + "\ncookies: " + string(cookieJson))

		// traefik ForwardAuth middleware should add X-Forwarded-Uri header
		if _, ok := r.Header["X-Forwarded-Uri"]; !ok {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		IPsAllowed := "73.53.171.231"
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" && strings.Contains(IPsAllowed, xff) {
			w.WriteHeader(http.StatusNoContent)
		} else {
			r.URL, _ = url.Parse(r.Header.Get("X-Forwarded-Uri"))
			r.Method = r.Header.Get("X-Forwarded-Method")
			r.Host = r.Header.Get("X-Forwarded-Host")
			headerBytes, _ := json.Marshal(r.Header)
			cookieMap := make(map[string]string)
			for _, c := range r.Cookies() {
				cookieMap[c.Name] = c.Value
			}
			cookieJson, _ := json.Marshal(cookieMap)
			fmt.Fprintln(w, "headers: "+string(headerBytes)+"\ncookies: "+string(cookieJson))
			w.WriteHeader(http.StatusForbidden)
		}
	})
}

//-------------------------------

func startServer(router *http.ServeMux, port int) {
	flag.StringVar(&listenAddr, "listen-addr", ":"+strconv.Itoa(port), "server listen address")
	flag.Parse()

	logger.Println("Server is starting...")

	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      tracing(nextRequestID)(logging(logger)(router)),
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		logger.Println("Server is shutting down...")
		atomic.StoreInt32(&healthy, 0)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()
	logger.Println("Server is ready to handle requests at", listenAddr)
	atomic.StoreInt32(&healthy, 1)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}
	<-done
	logger.Println("Server stopped")
}

func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Println(requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
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
