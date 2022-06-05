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

		dialogPvtIp_retCap_b32 := strings.Split(r.Host, ".")[0]
		dialogPvtIp_retCap, err := decode_b32l(dialogPvtIp_retCap_b32)
		Logger.Sugar().Debugf("dialogPvtIp_retCap_b32: %v, dialogPvtIp_retCap: %v", dialogPvtIp_retCap_b32, dialogPvtIp_retCap)
		if err != nil {
			Logger.Debug("decode_b32l(dialogPvtIp_retCap_b32) failed: " + err.Error())
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		dialogPvtIp_retCap_arr := strings.Split(dialogPvtIp_retCap, "|")
		if len(dialogPvtIp_retCap_arr) > 2 {
			Logger.Debug("unexpected dialogPvtIp_retCap: " + dialogPvtIp_retCap)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		dialogPvtIp := dialogPvtIp_retCap_arr[0]
		//TODO -- watch dialog pods, track their private ips to screen this

		retCap := ""
		if len(dialogPvtIp_retCap_arr) == 2 {
			retCap = dialogPvtIp_retCap_arr[1]
		}

		// host_in := strings.Split(r.Host, ".")[0]
		// Logger.Debug("host_in: " + host_in)

		urlStr := "https://" + dialogPvtIp + ":" + strings.Split(r.Host, ":")[1]
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
		r.Header.Set("x-ret-max-room-size", retCap)

		proxy.ServeHTTP(w, r)
	})
}

var b32l = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(-1)

func decode_b32l(s string) (string, error) {

	slice, err := b32l.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(slice), nil
}
