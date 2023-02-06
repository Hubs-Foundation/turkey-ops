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
	if r.Method == "PATCH" {

		customDomain := r.URL.Query().Get("customDomain")
		if !isCustomDomainGood(customDomain) {
			http.Error(w, "bad customDomain: "+customDomain, http.StatusBadRequest)
			return
		}
		err := Deployment_setLabel("custom-domain", customDomain)
		if err != nil {
			Logger.Error("failed to set custom-domain label on NS: " + err.Error())
			http.Error(w, "failed to set customDomain to NS: "+err.Error(), http.StatusInternalServerError)
			return
		}

		err = ret_AddSecondaryUrl(customDomain)
		if err != nil {
			http.Error(w, "failed @ ret_AddSecondaryUrl: "+err.Error(), http.StatusInternalServerError)
			return
		}

		letsencryptAcct := pickLetsencryptAccountForHubId()
		Logger.Sugar().Debugf("letsencryptAcct: %v", letsencryptAcct)
		err = runCertbotbotpod(letsencryptAcct)
		if err != nil {
			http.Error(w, "failed @ runCertbotbotpod: "+err.Error(), http.StatusInternalServerError)
			return
		}

		//add ingress route
		err = ingress_addCustomDomainRule(customDomain)
		if err != nil {
			http.Error(w, "failed @ ingress_addCustomDomainRule: "+err.Error(), http.StatusInternalServerError)
			return
		}
		//

		//refresh pods
		err = killPods("app=ita")
		if err != nil {
			http.Error(w, "failed to refresh ita pods: "+err.Error(), http.StatusInternalServerError)
			return
		}

		err = killPods("app=reticulum")
		if err != nil {
			http.Error(w, "failed to refresh reticulum pods: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "done")
	}

	http.Error(w, "", http.StatusMethodNotAllowed)

})

var UpdateCert = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/update-cert" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	letsencryptAcct := pickLetsencryptAccountForHubId()
	Logger.Sugar().Debugf("letsencryptAcct: %v", letsencryptAcct)
	err := runCertbotbotpod(letsencryptAcct)
	if err != nil {
		http.Error(w, "failed @ runCertbotbotpod: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "done")
})

func isCustomDomainGood(customDomain string) bool {

	if customDomain == "" {
		return false
	}

	//is valid domain with regex?

	//nslookup?

	return true
}
