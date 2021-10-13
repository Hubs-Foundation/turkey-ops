package internal

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

func dumpHeader(r *http.Request) string {
	headerBytes, _ := json.Marshal(r.Header)
	return string(headerBytes)
}

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

		logger.Debug("dumpHeader: " + dumpHeader(r))

		idp := r.URL.Query()["idp"]
		if len(idp) != 1 {
			logger.Sugar().Debug(`bad value for ["idp"] in r.URL.Query()`)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		client := GetClient(r)
		if client == "" {
			logger.Sugar().Debug(`bad value for ["client"] in r.URL.Query()`)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		email, err := CheckCookie(r)
		err = errors.New("fake error to force authRedirect, remove me after fxa's done")
		if err != nil {
			logger.Debug("valid auth cookie not found >>> authRedirect")
			authRedirect(w, r, idp[0])
		}

		// Valid request
		logger.Sugar().Debug("allowed. good cookie found for " + email)
		w.Header().Set("X-Forwarded-UserEmail", email)
		http.Redirect(w, r, client, http.StatusTemporaryRedirect)

	})
}

func authRedirect(w http.ResponseWriter, r *http.Request, providerName string) {

	provider, err := cfg.GetProvider(providerName)
	if err != nil {
		logger.Panic("internal.Cfg.GetProvider(" + providerName + ") failed: " + err.Error())
	}
	// Error indicates no cookie, generate nonce
	nonce, err := Nonce()
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

	// todo is there a better way to do this in go?
	redirectURL := "auth." + cfg.Domain
	if providerName == "google" {
		redirectURL = "https://auth." + cfg.Domain
	}

	loginURL := provider.GetLoginURL(redirectURL, MakeState(r, provider, nonce))
	logger.Debug(" ### loginURL: " + loginURL)
	http.Redirect(w, r, loginURL, http.StatusTemporaryRedirect)
}

func Logout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clear cookie
		http.SetCookie(w, ClearCookie(r))

		if cfg.LogoutRedirect != "" {
			logger.Debug("logout redirect to: " + cfg.LogoutRedirect)
			http.Redirect(w, r, cfg.LogoutRedirect, http.StatusTemporaryRedirect)
		} else {
			http.Error(w, "You have been logged out", http.StatusUnauthorized)
		}

	})
}

// oauth callback handler
func OauthFxa() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_fxa" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		logger.Info("Handling callback")
		logger.Info("dumpHeader: " + dumpHeader(r))

		// Check state
		state := r.URL.Query().Get("state")
		if err := ValidateState(state); err != nil {
			logger.Sugar().Warn("Error validating state: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Check for CSRF cookie
		c, err := FindCSRFCookie(r, state)
		if err != nil {
			logger.Sugar().Warn("Missing csrf cookie")
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Validate CSRF cookie against state
		valid, providerName, redirect, err := ValidateCSRFCookie(c, state)
		if !valid {
			logger.Sugar().Warn("Error validating csrf cookie: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Get provider
		p, err := cfg.GetProvider(providerName)
		if err != nil {
			logger.Sugar().Warn("Invalid provider in csrf cookie: " + providerName)
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Clear CSRF cookie
		http.SetCookie(w, ClearCSRFCookie(r, c))

		// Exchange code for token
		token, err := p.ExchangeCode("https://auth."+cfg.Domain+"/_oauth", r.URL.Query().Get("code"))
		if err != nil {
			logger.Sugar().Warn("Code exchange failed with provider: " + err.Error())
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		logger.Sugar().Debug("token", token)

		// Get user
		user, err := p.GetUser(token.AccessToken)
		if err != nil {
			logger.Sugar().Warn("Error getting user: " + err.Error())
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		logger.Sugar().Debug("user", user)

		// Generate cookie
		http.SetCookie(w, MakeCookie(r, user.Email))

		logger.Sugar().Debug("auth cookie generated: ", user)

		// Redirect
		w.Header().Set("X-Forwarded-UserName", user.Name)
		w.Header().Set("X-Forwarded-UserPicture", user.Picture)

		w.Header().Set("X-Forwarded-User", user.Email)
		http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)

	})
}

// oauth callback handler
func Oauth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_oauth" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		logger.Debug("Handling callback")
		// logger.Debug("dumpHeader: " + dumpHeader(r))

		// Check state
		state := r.URL.Query().Get("state")
		if err := ValidateState(state); err != nil {
			logger.Sugar().Warn("Error validating state: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Check for CSRF cookie
		c, err := FindCSRFCookie(r, state)
		if err != nil {
			logger.Sugar().Warn("Missing csrf cookie")
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Validate CSRF cookie against state
		valid, providerName, redirect, err := ValidateCSRFCookie(c, state)
		if !valid {
			logger.Sugar().Warn("Error validating csrf cookie: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Get provider
		p, err := cfg.GetProvider(providerName)
		if err != nil {
			logger.Sugar().Warn("Invalid provider in csrf cookie: " + providerName)
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Clear CSRF cookie
		http.SetCookie(w, ClearCSRFCookie(r, c))

		// Exchange code for token
		token, err := p.ExchangeCode("https://auth."+cfg.Domain+"/_oauth", r.URL.Query().Get("code"))
		if err != nil {
			logger.Sugar().Warn("Code exchange failed with provider: " + err.Error())
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		logger.Sugar().Debug("token", token)

		// Get user
		user, err := p.GetUser(token.AccessToken)
		if err != nil {
			logger.Sugar().Warn("Error getting user: " + err.Error())
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		logger.Sugar().Debug("user", user)

		// Generate cookie
		http.SetCookie(w, MakeCookie(r, user.Email))

		logger.Sugar().Debug("auth cookie generated: ", user)

		// Redirect
		w.Header().Set("X-Forwarded-UserName", user.Name)
		w.Header().Set("X-Forwarded-UserPicture", user.Picture)

		w.Header().Set("X-Forwarded-User", user.Email)
		http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)

	})
}

func Authn() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/authn" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		// Read URI from header if we're acting as forward auth middleware
		if _, ok := r.Header["X-Forwarded-Uri"]; ok {
			r.URL, _ = url.Parse(r.Header.Get("X-Forwarded-Uri"))
		}
		// logger.Debug("dumpHeader: " + dumpHeader(r))

		email, err := CheckCookie(r)
		if err != nil {
			logger.Debug("valid auth cookie not found >>> authRedirect")
			authRedirect(w, r, cfg.DefaultProvider)
		}

		logger.Sugar().Debug("allowed. good cookie found for " + email)
		w.Header().Set("X-Forwarded-UserEmail", email)

		clearCSRFcookies(w, r)

		w.WriteHeader(200)
	})
}

func clearCSRFcookies(w http.ResponseWriter, r *http.Request) {
	for _, c := range r.Cookies() {
		if strings.Index(c.Name[:11], "_turkeyauthcsrfcookie_") == 0 {
			http.SetCookie(w, &http.Cookie{Name: c.Name, Value: "", Path: "/", Expires: time.Unix(0, 0)})
		}
	}
}

// func TraefikIp() http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		if r.URL.Path != "/traefik-ip" {
// 			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 			return
// 		}
// 		if _, ok := r.Header["X-Forwarded-Uri"]; !ok {
// 			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
// 			return
// 		}

// 		IPsAllowed := os.Getenv("trusted_IPs") // "73.53.171.231"
// 		xff := r.Header.Get("X-Forwarded-For")
// 		if xff != "" && strings.Contains(IPsAllowed, xff) {
// 			w.WriteHeader(http.StatusNoContent)
// 		} else {
// 			logger.Info("not allowed !!! bad ip in xff: " + xff)
// 			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
// 		}
// 	})
// }
