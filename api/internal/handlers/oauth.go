package handlers

import (
	"main/internal"
	"net/http"

	"go.uber.org/zap"
)

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
		if err := internal.ValidateState(state); err != nil {
			logger.Info("Error validating state: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Check for CSRF cookie
		c, err := internal.FindCSRFCookie(r, state)
		if err != nil {
			logger.Info("Missing csrf cookie")
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Validate CSRF cookie against state
		valid, providerName, redirect, err := internal.ValidateCSRFCookie(c, state)
		if !valid {
			logger.Info("Error validating csrf cookie: " + err.Error())
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Get provider
		p, err := internal.Cfg.GetProvider(providerName)
		if err != nil {
			logger.Info("Invalid provider in csrf cookie: " + providerName)
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Clear CSRF cookie
		http.SetCookie(w, internal.ClearCSRFCookie(r, c))

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
		http.SetCookie(w, internal.MakeCookie(r, user.Email))

		logger.Info("auth cookie generated",
			zap.String("user.email", user.Email),
			zap.String("user.sub", user.Sub),
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
