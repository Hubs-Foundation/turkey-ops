package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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
	turkeyDomain            string
	googleOauthClientId     string
	googleOauthClientSecret string
)

func main() {

	turkeyDomain = "myhubs.net"
	googleOauthClientId = os.Getenv("oauthClientId_google")
	googleOauthClientSecret = os.Getenv("oauthClientSecret_google")

	router := http.NewServeMux()
	// router.Handle("/", root())
	router.Handle("/healthz", healthz())
	router.Handle("/traefik-ip", traefikIp())

	router.Handle("/login", login())

	router.Handle("/_oauth_google", _oauth_google())

	startServer(router, 9001)

}

func healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&healthy) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

func login() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		idp := r.URL.Query()["idp"][0]

		if idp == "google" {

			url := "https://accounts.google.com/o/oauth2/v2/auth?" +
				"scope=https%3A//www.googleapis.com/auth/drive.metadata.readonly&" +
				"access_type=offline&include_granted_scopes=true&" +
				"response_type=code&state=state_parameter_passthrough_value&" +
				"redirect_uri=https%3A//auth." + turkeyDomain + "/code&" +
				"client_id=" + googleOauthClientId
			http.Redirect(w, r, url, http.StatusSeeOther)
		}
	})
}

func _oauth_google() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path != "/_oauth_google" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		code := r.URL.Query()["code"][0]

		fmt.Println("### /_oauth_google ~~~ received code: " + code)

		fmt.Println("### /_oauth_google ~~~ dumping r !!!")
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

		//Step 5: Exchange authorization code for refresh and access tokens
		//https://developers.google.com/identity/protocols/oauth2/web-server#exchange-authorization-code
		req, err := http.NewRequest("POST", "https://oauth2.googleapis.com/token", nil)
		if err != nil {
			panic("Exchange authorization code for refresh and access tokens FAILED: " + err.Error())
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("code", code)
		req.Header.Set("client_id", googleOauthClientId)
		req.Header.Set("client_secret", googleOauthClientSecret)
		req.Header.Set("redirect_uri", "https://portal.myhubs.net")
		req.Header.Set("grant_type", "authorization_code")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		fmt.Println("response Headers:", resp.Header)
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("response Body:", string(body))

		respCookieMap := make(map[string]string)
		for _, c := range resp.Cookies() {
			respCookieMap[c.Name] = c.Value
		}
		cookieJson, _ = json.Marshal(respCookieMap)
		fmt.Println("response cookie: " + string(cookieJson))

	})
}

func traefikIp() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/traefik-ip" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		if _, ok := r.Header["X-Forwarded-Uri"]; !ok {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		// r.URL, _ = url.Parse(r.Header.Get("X-Forwarded-Uri"))
		// r.Method = r.Header.Get("X-Forwarded-Method")
		// r.Host = r.Header.Get("X-Forwarded-Host")
		// headerBytes, _ := json.Marshal(r.Header)
		// cookieMap := make(map[string]string)
		// for _, c := range r.Cookies() {
		// 	cookieMap[c.Name] = c.Value
		// }
		// cookieJson, _ := json.Marshal(cookieMap)
		// fmt.Println("headers: " + string(headerBytes) + "\ncookies: " + string(cookieJson))

		// traefik ForwardAuth middleware should add X-Forwarded-Uri header

		IPsAllowed := os.Getenv("trusted_IPs") // "73.53.171.231"
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" && strings.Contains(IPsAllowed, xff) {
			w.WriteHeader(http.StatusNoContent)
		} else {
			fmt.Println("##################### not allowed !!! bad ip in xff: " + xff)
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
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
