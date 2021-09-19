package handlers

import (
	"main/internal"
	"net/http"
)

func Login() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		idp := r.URL.Query()["idp"]

		// Get auth cookie
		c, err := r.Cookie(internal.Cfg.CookieName)
		if err != nil {
			authRedirect(w, r, idp[0])
			return
		}

		// Validate cookie
		email, err := internal.ValidateCookie(r, c)
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

func authRedirect(w http.ResponseWriter, r *http.Request, providerName string) {

	p, err := internal.Cfg.GetProvider(providerName)
	if err != nil {
		logger.Panic("internal.Cfg.GetProvider(" + providerName + ") failed: " + err.Error())
	}
	// Error indicates no cookie, generate nonce
	err, nonce := internal.Nonce()
	if err != nil {
		logger.Info("Error generating nonce: " + err.Error())
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Set the CSRF cookie
	csrf := internal.MakeCSRFCookie(r, nonce)
	http.SetCookie(w, csrf)

	if !internal.Cfg.InsecureCookie && r.Header.Get("X-Forwarded-Proto") != "https" {
		logger.Info("You are using \"secure\" cookies for a request that was not " +
			"received via https. You should either redirect to https or pass the " +
			"\"insecure-cookie\" config option to permit cookies via http.")
	}

	loginURL := p.GetLoginURL("https://auth."+cfg.Domain+"/_oauth", internal.MakeState(r, p, nonce))

	http.Redirect(w, r, loginURL, http.StatusTemporaryRedirect)

}
