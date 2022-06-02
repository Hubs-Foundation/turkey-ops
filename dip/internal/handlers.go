package internal

import (
	"encoding/base32"
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
		host_in := strings.Split(r.URL.Host, ".")[0]

		urlStr := decode_b32l(host_in)
		Logger.Debug("urlStr: " + urlStr)

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

var b32l = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567")

func decode_b32l(s string) string {
	if i := len(s) % 4; i != 0 {
		s += strings.Repeat("=", 4-i)
	}
	slice, err := b32l.DecodeString(s)
	if err != nil {
		Logger.Error(err.Error())
	}
	return string(slice)
}
