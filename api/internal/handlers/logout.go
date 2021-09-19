package handlers

import (
	"main/internal"
	"net/http"
)

func Logout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clear cookie
		http.SetCookie(w, internal.ClearCookie(r))

		if internal.Cfg.LogoutRedirect != "" {
			logger.Info("logout redirect to: " + cfg.LogoutRedirect)
			http.Redirect(w, r, cfg.LogoutRedirect, http.StatusTemporaryRedirect)
		} else {
			http.Error(w, "You have been logged out", http.StatusUnauthorized)
		}

	})
}
