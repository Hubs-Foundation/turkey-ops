package handlers

import (
	"net/http"
	"os"
	"strings"
)

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
