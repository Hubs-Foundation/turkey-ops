package internal

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
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

var Healthy int32

type Server struct {
	port         int
	isHttps      bool
	router       *http.ServeMux
	listenAddr   string
	requestIDKey key
}

func NewServer(router *http.ServeMux, port int, isHttps bool) *Server {
	var server = &Server{
		router:       router,
		port:         port,
		isHttps:      isHttps,
		requestIDKey: 0,
	}
	return server
}

func StartNewServer(router *http.ServeMux, port int, isHttps bool) *Server {
	var server = &Server{
		router:       router,
		listenAddr:   ":" + strconv.Itoa(port),
		isHttps:      isHttps,
		requestIDKey: 0,
	}

	server.Start()

	return server
}

func (s *Server) Start() {

	Logger.Debug("Server is starting...")

	nextRequestID := func() string {
		// return fmt.Sprintf("%d", time.Now().UnixNano())
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(time.Now().UnixNano()))
		return base64.RawStdEncoding.EncodeToString(b)
	}

	server := &http.Server{
		Addr:         s.listenAddr,
		Handler:      s.tracing(nextRequestID)(s.logging()(s.router)),
		ReadTimeout:  600 * time.Minute,
		WriteTimeout: 600 * time.Minute,
		IdleTimeout:  60 * time.Minute,
	}
	if s.isHttps {
		server.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		}

		// haproxy has problem with h2+tls
		// (http2: server: error reading preface from client ... read: connection reset by peer)
		// disabling h2 here for now
		// TODO: figure it out, probably need new haproxy version
		server.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
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
	Logger.Debug("Server is ready to handle requests at: " + s.listenAddr)
	atomic.StoreInt32(&Healthy, 1)

	if s.isHttps {
		if err := server.ListenAndServeTLS("cert.pem", "key.pem"); err != nil && err != http.ErrServerClosed {
			Logger.Sugar().Fatalf("Could not listen on %s: %v\n", s.listenAddr, err)
		}
	} else {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Logger.Sugar().Fatalf("Could not listen on %s: %v\n", s.listenAddr, err)
		}
	}
	<-done
	Logger.Debug("Server stopped")
}
func (s *Server) tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), s.requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
func (s *Server) logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(s.requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				Logger.Debug("(" + requestID + "):" + r.Method + "@" + r.URL.Path + " <- " + r.RemoteAddr)
			}()
			next.ServeHTTP(w, r)
		})
	}
}
