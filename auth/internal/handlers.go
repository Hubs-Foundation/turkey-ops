package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"main/internal/idp"
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

		Logger.Debug("dumpHeader: " + dumpHeader(r))

		idp := r.URL.Query()["idp"]
		if len(idp) != 1 {
			Logger.Sugar().Debug(`bad value for ["idp"] in r.URL.Query()`)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		client := GetClient(r)
		if client == "" {
			Logger.Sugar().Debug(`bad value for ["client"] in r.URL.Query()`)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		email, err := CheckCookie(r)
		// err = errors.New("fake error to force authRedirect, remove me after fxa's done")
		if err != nil {
			Logger.Debug("valid auth cookie not found >>> authRedirect ... err: " + err.Error())
			authRedirect(w, r, idp[0])
		}

		// Valid request
		Logger.Sugar().Debug("allowed. good cookie found for " + email)
		w.Header().Set("X-Forwarded-UserEmail", email)
		w.Header().Set("X-Forwarded-Idp", cfg.DefaultProvider)

		http.Redirect(w, r, client, http.StatusTemporaryRedirect)
	})
}

func authRedirect(w http.ResponseWriter, r *http.Request, providerName string) {

	provider, err := cfg.GetProvider(providerName)
	if err != nil {
		Logger.Panic("internal.Cfg.GetProvider(" + providerName + ") failed: " + err.Error())
	}
	// Error indicates no cookie, generate nonce
	nonce, err := Nonce()
	if err != nil {
		Logger.Info("Error generating nonce: " + err.Error())
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Set the CSRF cookie
	csrf := MakeCSRFCookie(r, nonce)
	http.SetCookie(w, csrf)

	// if !cfg.InsecureCookie && r.Header.Get("X-Forwarded-Proto") != "https" {
	// 	Logger.Info("You are using \"secure\" cookies for a request that was not " + "received via https. You should either redirect to https or pass the " + "\"insecure-cookie\" config option to permit cookies via http.")
	// }

	// todo is there a better way to do this in go?
	redirectURL := "auth." + cfg.Domain
	if providerName == "google" {
		redirectURL = "https://auth." + cfg.Domain
	}

	loginURL := provider.GetLoginURL(redirectURL, MakeState(r, provider, nonce))
	Logger.Debug(" ### loginURL: " + loginURL)
	http.Redirect(w, r, loginURL, http.StatusTemporaryRedirect)
}

func Logout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clear cookie
		http.SetCookie(w, ClearCookie(r, cfg.JwtCookieName))
		http.SetCookie(w, ClearCookie(r, cfg.CookieName))

		if cfg.LogoutRedirect != "" {
			Logger.Debug("logout redirect to: " + cfg.LogoutRedirect)
			http.Redirect(w, r, cfg.LogoutRedirect, http.StatusTemporaryRedirect)
		} else {
			http.Error(w, "You have been logged out", http.StatusUnauthorized)
		}

	})
}

// oauth callback handler
func Oauth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_oauth" && r.URL.Path != "/_fxa" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		Logger.Debug("Handling callback")
		Logger.Debug("dumpHeader: " + dumpHeader(r))

		// Check state
		state := r.URL.Query().Get("state")
		if err := ValidateState(state); err != nil {
			Logger.Sugar().Warn("Error validating state: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Check for CSRF cookie
		c, err := FindCSRFCookie(r, state)
		if err != nil {
			Logger.Sugar().Warn("Missing csrf cookie")
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Validate CSRF cookie against state
		valid, providerName, redirect, err := ValidateCSRFCookie(c, state)
		if !valid {
			Logger.Sugar().Warn("Error validating csrf cookie: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Get provider
		p, err := cfg.GetProvider(providerName)
		if err != nil {
			Logger.Sugar().Warn("Invalid provider in csrf cookie: " + providerName)
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Clear CSRF cookie
		http.SetCookie(w, ClearCSRFCookie(r, c))

		// Exchange code for token
		code := r.URL.Query().Get("code")
		Logger.Debug("Exchange code (" + code + ") for token")
		token, err := p.ExchangeCode("https://auth."+cfg.Domain+"/_oauth", code)
		if err != nil {
			Logger.Sugar().Warn("Code exchange failed with provider (" + providerName + "): " + err.Error())
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		Logger.Sugar().Debug("token: ", token)

		// Get user
		user, err := p.GetUser(token.AccessToken)
		if err != nil {
			Logger.Sugar().Warn("Error getting user: " + err.Error())
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		Logger.Sugar().Debug("user", user)

		// Generate cookie
		jwtCookie, err := MakeJwtCookie(r, user)
		if err != nil {
			Logger.Sugar().Errorf("failed to make cookie for user: %v", user)
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		http.SetCookie(w, jwtCookie)

		//dev only -- make an auth cookie too
		if cfg.Env == "dev" {
			http.SetCookie(w, MakeAuthCookie(r, user.Email))
		}

		Logger.Sugar().Debug("cookie generated for: ", user)

		// Redirect
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
		// Logger.Debug("dumpHeader: " + dumpHeader(r))
		Logger.Sugar().Debugf("r.Header, %v", r.Header)
		// Read URI from header if we're acting as forward auth middleware
		// because traefik would add X-Forwarded-Uri for original requestor
		if _, ok := r.Header["X-Forwarded-Uri"]; ok {
			r.URL, _ = url.Parse(r.Header.Get("X-Forwarded-Uri"))
		}

		email, err := CheckCookie(r)
		if err != nil {
			Logger.Debug("valid auth cookie not found >>> authRedirect")
			authRedirect(w, r, cfg.DefaultProvider)
			return
		}

		Logger.Sugar().Debug("allowed. good cookie found for " + email)
		w.Header().Set("X-Forwarded-UserEmail", email)
		w.Header().Set("X-Forwarded-Idp", cfg.DefaultProvider)

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

func AuthnProxy() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		Logger.Sugar().Debugf("request dump: %v", r)

		backend, ok := r.Header["Backend"]
		if !ok || len(backend) != 1 {
			Logger.Sugar().Debugf("bad r.Headers[backend]")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		urlStr := backend[0]
		backendUrl, err := url.Parse(urlStr)
		if err != nil {
			Logger.Sugar().Errorf("bad r.Headers[backend], (%v) because %v", urlStr, err.Error())
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		// if r.URL.Path == "/" { //no direct calls, i can't tell host but i can tell path
		// 	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		// 	return
		// }
		if r.Header.Get("x-turkeyauth-proxied") != "" {
			Logger.Error("omg authn proxy's looping, why???")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		Logger.Sugar().Debugf("backend url: %v", backendUrl)

		email, err := CheckCookie(r)
		if err != nil {
			Logger.Debug("valid auth cookie not found >>> authRedirect")
			r.URL.Host = backendUrl.Host
			authRedirect(w, r, cfg.DefaultProvider)
			return
		}
		Logger.Sugar().Debug("allowed. good cookie found for " + email)

		// proxy := httputil.NewSingleHostReverseProxy(backendUrl)
		// proxy.Transport = &http.Transport{ResponseHeaderTimeout: 1 * time.Minute}
		proxy, err := Proxyman.Get(urlStr)
		if err != nil {
			Logger.Error("get proxy failed: " + err.Error())
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		r.Header.Set("X-Forwarded-UserEmail", email)
		r.Header.Set("x-turkeyauth-proxied", "1")

		proxy.ServeHTTP(w, r)
	})
}

// func modifyRequest(req *http.Request, headers map[string]string) {
// 	for k, v := range headers {
// 		req.Header.Set(k, v)
// 	}
// }

func ChkCookie() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chk_cookie" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		Logger.Sugar().Debugf("r.Header, %v", r.Header)

		email, err := CheckCookie(r)
		if err != nil {
			Logger.Debug("bad cookie" + err.Error())
			w.WriteHeader(http.StatusForbidden)
			return
		}
		Logger.Sugar().Debug("good cookie found for " + email)

		// accessing := r.URL.Query()["req"]

		clearCSRFcookies(w, r)

		// json.NewEncoder(w).Encode(map[string]interface{}{
		// 	"user_email": email,
		// 	// "user_role":  "notyet",
		// })

		w.Header().Add("verified-UserEmail", email)
		w.WriteHeader(http.StatusOK)
	})
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
// 			Logger.Info("not allowed !!! bad ip in xff: " + xff)
// 			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
// 		}
// 	})
// }

func GimmieTestJwtCookie() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gimmie_test_jwt_cookie" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		Logger.Debug("dumpHeader: " + dumpHeader(r))
		Logger.Sugar().Debugf("r.RemoteAddr: %v", r.RemoteAddr)
		Logger.Sugar().Debugf("r.RequestURI: %v", r.RequestURI)

		rBodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			Logger.Error("bad body: " + string(rBodyBytes))
			http.Error(w, "bad body: "+string(rBodyBytes), http.StatusBadRequest)
			return
		}
		var user idp.User
		err = json.Unmarshal(rBodyBytes, &user)
		if err != nil {
			Logger.Error("failed to unmarshal body to user: " + err.Error())
			http.Error(w, "failed to unmarshal body to user: "+err.Error(), http.StatusBadRequest)
			return
		}

		jwtCookie, err := MakeJwtCookie(r, user)
		if err != nil {
			Logger.Sugar().Errorf("failed to make cookie for user: %v", user)
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}

		fmt.Fprintf(w, jwtCookie.Value)
	})
}
