package internal

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
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

func Proxy() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		Logger.Sugar().Debugf("request dump: %v", r)

		r.URL.Host = strings.Replace(r.URL.Host, "stream.", "dialog.", 1)

		urlStr := fmt.Sprintf("%v", r.URL)
		backendUrl, err := url.Parse(urlStr)
		if err != nil {
			Logger.Sugar().Errorf("bad r.Headers[backend], (%v) because %v", urlStr, err.Error())
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		if r.URL.Path == "/" { //no direct calls, i can't tell host but i can tell path
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		if r.Header.Get("x-turkeyauth-proxied") != "" {
			Logger.Error("omg authn proxy's looping, why???")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		Logger.Sugar().Debugf("backend url: %v", backendUrl)

		proxy, err := Proxyman.Get(urlStr)
		if err != nil {
			Logger.Error("get proxy failed: " + err.Error())
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		r.Header.Set("x-turkey-proxied", "1")

		proxy.ServeHTTP(w, r)
	})
}
