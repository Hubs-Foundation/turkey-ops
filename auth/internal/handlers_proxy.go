package internal

import (
	"net/http"
	"net/url"
	"strings"
)

var inta = 6

var authorizedUsers = map[string]string{
	"107.139.213.232": "gtan@mozilla.com",
	"222.153.126.253": "you@mozilla.com",
}

func AuthnProxy() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		Logger.Sugar().Debugf("dump r, %v", r)

		clientIP := r.Header.Get("X-Forwarded-For")
		if _, ok := authorizedUsers[clientIP]; !ok {
			Logger.Debug("unauthorized clientIP: " + clientIP)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		if r.URL.Path == "/" {
			Logger.Debug("direct calls not allowed")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/turkeyauthproxy") {
			r.URL.Path = strings.Replace(r.URL.Path, "/turkeyauthproxy", "", 1)
			Logger.Sugar().Debugf(" path: %v", r)
		}

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

		if r.Header.Get("x-turkeyauth-proxied") != "" {
			Logger.Error("omg authn proxy's looping, why???")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		r.Header.Set("x-turkeyauth-proxied", "1")

		// Logger.Sugar().Debugf("backend url: %v", backendUrl)

		// email, err := CheckCookie(r)
		// if err != nil {
		authcookieName := "tap|" + r.Host
		data, err := checkAuthCookie(r, authcookieName)

		Logger.Sugar().Debugf("~~~checkAuthCookie (name: %v), email: %v, err: %v", authcookieName, data, err)

		email := strings.Split(data, "#")[0]

		if err != nil {
			Logger.Debug("valid auth cookie not found >>> authRedirect")
			r.URL.Host = backendUrl.Host
			authRedirect(w, r, cfg.DefaultProvider)
			return
		}

		//todo: need a better authz here
		AllowedEmailDomains := r.Header.Get("AllowedEmailDomains")
		if AllowedEmailDomains != "" {
			emailDomain := strings.Split(email, "@")[1]
			if !strings.Contains(AllowedEmailDomains, emailDomain+",") {
				// Logger.Sugar().Debugf("unauthorized email: %v, AllowedEmailDomains: %v >>> authRedirect", email, AllowedEmailDomains)
				// authRedirect(w, r, cfg.DefaultProvider)
				// http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
			// Logger.Sugar().Debugf("ALLOWED: %v, %v", email, urlStr)
		}
		AllowedEmails := r.Header.Get("TAP_AllowedEmails")
		if AllowedEmails != "" {
			if !strings.Contains(AllowedEmails, email) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
		}

		proxy, err := Proxyman.Get(urlStr)
		if err != nil {
			Logger.Error("get proxy failed: " + err.Error())
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		r.Header.Set("X-Forwarded-UserEmail", email)

		proxy.ServeHTTP(w, r)
	})
}
