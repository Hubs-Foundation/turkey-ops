package main

import (
	"context"
	"flag"
	"fmt"
	"main/internal"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"

	"strconv"

	"go.uber.org/zap"
)

type key int

const (
	requestIDKey key = 0
)

var (
	listenAddr string
	healthy    int32

	Logger *zap.Logger

	turkeyDomain string
)

func main() {
	turkeyDomain = "myhubs.net"

	Logger, _ = zap.NewProduction()
	internal.MakeCfg(Logger)

	router := http.NewServeMux()
	// router.Handle("/", root())
	router.Handle("/healthz", healthz())
	router.Handle("/traefik-ip", traefikIp())

	router.Handle("/login", login())
	router.Handle("/logout", logout())

	// oauth callback catcher
	router.Handle("/_oauth", _oauth())

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

		idp := r.URL.Query()["idp"]

		// Get auth cookie
		c, err := r.Cookie(internal.Cfg.CookieName)
		if err != nil {
			authRedirect(w, r, idp[0])
			return
		}

		// Validate cookie
		email, err := internal.ValidateCookie(r, c)
		if err != nil {
			if err.Error() == "Cookie has expired" {
				Logger.Info("Cookie has expired")
				authRedirect(w, r, idp[0])
			} else {
				Logger.Info("Invalid cookie, err: " + err.Error())
				http.Error(w, "Not authorized", http.StatusUnauthorized)
			}
			return
		}

		// // Validate user ########## do i want authZ here ????
		// valid := internal.ValidateEmail(email, "rule")
		// if !valid {
		// 	logger.Println("Invalid email: " + email)
		// 	http.Error(w, "Not authorized", 401)
		// 	return
		// }

		// Valid request
		Logger.Info("Allowing valid request")
		w.Header().Set("X-Forwarded-User", email)
		w.WriteHeader(200)

	})
}
func logout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clear cookie
		http.SetCookie(w, internal.ClearCookie(r))

		if internal.Cfg.LogoutRedirect != "" {
			Logger.Info("logout redirect to: " + internal.Cfg.LogoutRedirect)
			http.Redirect(w, r, internal.Cfg.LogoutRedirect, http.StatusTemporaryRedirect)
		} else {
			http.Error(w, "You have been logged out", http.StatusUnauthorized)
		}

	})
}

func authRedirect(w http.ResponseWriter, r *http.Request, providerName string) {

	p, err := internal.Cfg.GetProvider(providerName)
	if err != nil {
		Logger.Panic("internal.Cfg.GetProvider(" + providerName + ") failed: " + err.Error())
	}
	// Error indicates no cookie, generate nonce
	err, nonce := internal.Nonce()
	if err != nil {
		Logger.Info("Error generating nonce: " + err.Error())
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Set the CSRF cookie
	csrf := internal.MakeCSRFCookie(r, nonce)
	http.SetCookie(w, csrf)

	if !internal.Cfg.InsecureCookie && r.Header.Get("X-Forwarded-Proto") != "https" {
		Logger.Info("You are using \"secure\" cookies for a request that was not " +
			"received via https. You should either redirect to https or pass the " +
			"\"insecure-cookie\" config option to permit cookies via http.")
	}

	loginURL := p.GetLoginURL("https://auth."+turkeyDomain+"/_oauth", internal.MakeState(r, p, nonce))

	http.Redirect(w, r, loginURL, http.StatusTemporaryRedirect)

}

// oauth callback handler
func _oauth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_oauth" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		Logger.Info("Handling callback")
		// Check state
		state := r.URL.Query().Get("state")
		if err := internal.ValidateState(state); err != nil {
			Logger.Info("Error validating state: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Check for CSRF cookie
		c, err := internal.FindCSRFCookie(r, state)
		if err != nil {
			Logger.Info("Missing csrf cookie")
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Validate CSRF cookie against state
		valid, providerName, redirect, err := internal.ValidateCSRFCookie(c, state)
		if !valid {
			Logger.Info("Error validating csrf cookie: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// // Get provider
		// p, err := internal.Cfg.GetConfiguredProvider(providerName)
		p, err := internal.Cfg.GetProvider(providerName)
		if err != nil {
			Logger.Info("Invalid provider in csrf cookie: " + providerName)
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Clear CSRF cookie
		http.SetCookie(w, internal.ClearCSRFCookie(r, c))

		// Exchange code for token
		token, err := p.ExchangeCode("https://auth."+turkeyDomain+"/_oauth", r.URL.Query().Get("code"))
		if err != nil {
			Logger.Info("Code exchange failed with provider: " + err.Error())
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}

		Logger.Info("accessToken: " + token.AccessToken)
		Logger.Info("idToken: " + token.IdToken)

		// Get user
		user, err := p.GetUser(token.AccessToken)
		if err != nil {
			Logger.Info("Error getting user: " + err.Error())
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}

		// Generate cookie
		http.SetCookie(w, internal.MakeCookie(r, user.Email))
		// logger.WithFields(logrus.Fields{
		// 	"provider": providerName,
		// 	"redirect": redirect,
		// 	"user":     user.Email,
		// }).Info("Successfully generated auth cookie, redirecting user.")
		Logger.Info("auth cookie generated",
			zap.String("user.email", user.Email),
			zap.String("provider", providerName),
			zap.String("redirect", redirect),
		)

		// Redirect
		http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)

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
			Logger.Info("not allowed !!! bad ip in xff: " + xff)
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		}
	})
}

//-------------------------------

func startServer(router *http.ServeMux, port int) {
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
		atomic.StoreInt32(&healthy, 0)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			Logger.Sugar().Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()
	Logger.Info("Server is ready to handle requests at" + listenAddr)
	atomic.StoreInt32(&healthy, 1)
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
