package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"go.uber.org/zap"
)

func Healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&Healthy) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

func Login() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		headerBytes, _ := json.Marshal(r.Header)
		fmt.Println(string(headerBytes))

		idp := r.URL.Query()["idp"]

		// Get auth cookie
		c, err := r.Cookie(cfg.CookieName)
		if err != nil {
			authRedirect(w, r, idp[0])
			return
		}

		// Validate cookie
		email, err := ValidateCookie(r, c)
		if err != nil {
			if err.Error() == "Cookie has expired" {
				logger.Info("Cookie has expired")
				authRedirect(w, r, idp[0])
			} else {
				logger.Info("Invalid cookie, err: " + err.Error())
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
		logger.Info("Allowing valid request")
		w.Header().Set("X-Forwarded-User", email)
		w.WriteHeader(200)

	})
}

func Logout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clear cookie
		http.SetCookie(w, ClearCookie(r))

		if cfg.LogoutRedirect != "" {
			logger.Info("logout redirect to: " + cfg.LogoutRedirect)
			http.Redirect(w, r, cfg.LogoutRedirect, http.StatusTemporaryRedirect)
		} else {
			http.Error(w, "You have been logged out", http.StatusUnauthorized)
		}

	})
}

// oauth callback handler
func Oauth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_oauth" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		logger.Info("Handling callback")
		// Check state
		state := r.URL.Query().Get("state")
		if err := ValidateState(state); err != nil {
			logger.Info("Error validating state: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Check for CSRF cookie
		c, err := FindCSRFCookie(r, state)
		if err != nil {
			logger.Info("Missing csrf cookie")
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Validate CSRF cookie against state
		valid, providerName, redirect, err := ValidateCSRFCookie(c, state)
		if !valid {
			logger.Info("Error validating csrf cookie: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Get provider
		p, err := cfg.GetProvider(providerName)
		if err != nil {
			logger.Info("Invalid provider in csrf cookie: " + providerName)
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Clear CSRF cookie
		http.SetCookie(w, ClearCSRFCookie(r, c))

		// Exchange code for token
		token, err := p.ExchangeCode("https://auth."+cfg.Domain+"/_oauth", r.URL.Query().Get("code"))
		if err != nil {
			logger.Info("Code exchange failed with provider: " + err.Error())
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		logger.Sugar().Debug("token", token)

		// Get user
		user, err := p.GetUser(token.AccessToken)
		if err != nil {
			logger.Info("Error getting user: " + err.Error())
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		logger.Sugar().Debug("user", user)

		// Generate cookie
		http.SetCookie(w, MakeCookie(r, user.Email))

		logger.Info("auth cookie generated",
			zap.String("user.email", user.Email),
			zap.String("user.sub", user.Id),
			zap.String("user.name", user.Name),
			zap.String("user.picture", user.Picture),
			zap.String("user.locale", user.Locale),
			zap.String("provider", providerName),
			zap.String("redirect", redirect),
		)

		// Redirect
		http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)

	})
}

func TraefikIp() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/traefik-ip" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		if _, ok := r.Header["X-Forwarded-Uri"]; !ok {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		IPsAllowed := os.Getenv("trusted_IPs") // "73.53.171.231"
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" && strings.Contains(IPsAllowed, xff) {
			w.WriteHeader(http.StatusNoContent)
		} else {
			logger.Info("not allowed !!! bad ip in xff: " + xff)
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		}
	})
}

func authRedirect(w http.ResponseWriter, r *http.Request, providerName string) {

	p, err := cfg.GetProvider(providerName)
	if err != nil {
		logger.Panic("internal.Cfg.GetProvider(" + providerName + ") failed: " + err.Error())
	}
	// Error indicates no cookie, generate nonce
	err, nonce := Nonce()
	if err != nil {
		logger.Info("Error generating nonce: " + err.Error())
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Set the CSRF cookie
	csrf := MakeCSRFCookie(r, nonce)
	http.SetCookie(w, csrf)

	if !cfg.InsecureCookie && r.Header.Get("X-Forwarded-Proto") != "https" {
		logger.Info("You are using \"secure\" cookies for a request that was not " + "received via https. You should either redirect to https or pass the " + "\"insecure-cookie\" config option to permit cookies via http.")
	}

	loginURL := p.GetLoginURL("https://auth."+cfg.Domain+"/_oauth", MakeState(r, p, nonce))

	http.Redirect(w, r, loginURL, http.StatusTemporaryRedirect)

}
