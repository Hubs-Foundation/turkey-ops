package internal

import (
	"fmt"
	"net/http"
)

var CustomDomain = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/custom-domain" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	customDomain := r.Header.Get("custom-domain")
	err := NS_setLabel("custom-domain", customDomain)
	if err != nil {
		Logger.Error("failed to set custom-domain label on NS: " + err.Error())
	}

	ret_AddSecondaryUrl(customDomain)

	letsencryptAcct := pickLetsencryptAccountForHubId()
	Logger.Sugar().Debugf("letsencryptAcct: %v", letsencryptAcct)

	// err := k8s_removeNfsMount("hubs")
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "done")
})

var UpdateCert = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/update-cert" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	customDomain := r.Header.Get("custom-domain")

	ret_AddSecondaryUrl(customDomain)

	letsencryptAcct := pickLetsencryptAccountForHubId()
	Logger.Sugar().Debugf("letsencryptAcct: %v", letsencryptAcct)

	runCertbotbotpod(letsencryptAcct)
	// err := k8s_removeNfsMount("hubs")
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "done")
})
