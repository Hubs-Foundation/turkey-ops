package internal

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		//certbotbot
		letsencryptAcct := pickLetsencryptAccountForHubId()
		Logger.Sugar().Debugf("letsencryptAcct: %v", letsencryptAcct)
		err = runCertbotbotpod(letsencryptAcct, customDomain)
		if err != nil {
			http.Error(w, "failed @ runCertbotbotpod: "+err.Error(), http.StatusInternalServerError)
			return
		}

		err = setCustomDomain(customDomain)
		if err != nil {
			http.Error(w, "failed @ setCustomDomain: "+err.Error(), http.StatusInternalServerError)
			return
		}

		//refresh ret pods
		err = killPods("app=reticulum")
		if err != nil {
			http.Error(w, "failed to refresh reticulum pods: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// err = killPods("app=ita")
		// if err != nil {
		// 	http.Error(w, "failed to refresh ita pods: "+err.Error(), http.StatusInternalServerError)
		// 	return
		// }
		//update features
		cfg.makeFeatures()

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
	err := runCertbotbotpod(letsencryptAcct, "")
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

func setCustomDomain(customDomain string) error {

	//update ret config
	retCm, err := cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Get(context.Background(), "ret-config", metav1.GetOptions{})
	if err != nil {
		return err
	}
	retCm.Data["config.toml.template"] =
		strings.Replace(
			retCm.Data["config.toml.template"], "<SUB_DOMAIN>.<HUB_DOMAIN>", customDomain, 1)
	_, err = cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Update(context.Background(), retCm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	//update hubs and spoke's env var
	for _, appName := range []string{"hubs", "spoke"} {
		d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), appName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for i, env := range d.Spec.Template.Spec.Containers[0].Env {
			d.Spec.Template.Spec.Containers[0].Env[i].Value =
				strings.Replace(env.Value, cfg.SubDomain+"."+cfg.HubDomain, customDomain, -1)
		}
		_, err = cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	//add ingress route -- todo: replace it?
	err = ingress_addCustomDomainRule(customDomain)
	if err != nil {
		return err
	}

	err = ingress_updateHaproxyCors("https://" + customDomain)

	return err
}

func ingress_addCustomDomainRule(customDomain string) error {
	igs, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	ig, retRootRule, err := findIngressWithRetRootRule(&igs.Items)
	if err != nil {
		Logger.Error("findIngressWithRetRootPath failed: " + err.Error())
		return err
	}
	if ingressRuleAlreadyCreated_byBackendHost(ig, customDomain) { // ingressRuleAlreadyCreated
		return nil
	}
	customDomainRule := retRootRule.DeepCopy()
	customDomainRule.Host = customDomain
	ig.Spec.Rules = append(ig.Spec.Rules, *customDomainRule)

	ig.Spec.TLS = append(ig.Spec.TLS, networkingv1.IngressTLS{
		Hosts:      []string{customDomain},
		SecretName: "cert_" + customDomain,
	})

	newIg, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Update(context.Background(), ig, metav1.UpdateOptions{})
	if err != nil {
		Logger.Sugar().Errorf("failed to update ingress with customDomainRule: %v", err)
		return err
	}
	Logger.Sugar().Debugf("updated ingress: %v", newIg)
	return nil
}

func ingress_updateHaproxyCors(origins string) error {
	igs, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, ig := range igs.Items {
		ig.Annotations["haproxy.org/response-set-header"] = origins
		_, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Update(context.Background(), &ig, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
