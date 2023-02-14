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

		fromDomain := r.URL.Query().Get("from_domain")
		toDomain := r.URL.Query().Get("to_domain")

		Logger.Sugar().Debugf("received: fromDomain: %v, toDomain: %v", fromDomain, toDomain)

		if fromDomain == "" && toDomain == "" {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		if fromDomain != "" {
			if current, _ := Deployment_getLabel("custom-domain"); fromDomain != current {
				http.Error(w, fmt.Sprintf("expected fromDomain %v, expecting: %v", fromDomain, current), http.StatusBadRequest)
				return
			}
		}

		if toDomain != "" { //certbotbot
			letsencryptAcct := pickLetsencryptAccountForHubId()
			Logger.Sugar().Debugf("letsencryptAcct: %v", letsencryptAcct)
			err := runCertbotbotpod(letsencryptAcct, toDomain)
			if err != nil {
				http.Error(w, "failed @ runCertbotbotpod: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		Logger.Sugar().Debugf("calling setCustomDomain with fromDomain: %v, toDomain: %v", fromDomain, toDomain)
		err := setCustomDomain(fromDomain, toDomain)
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

		err = Deployment_setLabel("custom-domain", toDomain)

		//update features
		cfg.Features.enableCustomClient()
		defer cfg.Features.setupFeatures()

		if err != nil {
			Logger.Error("failed to set custom-domain label on NS: " + err.Error())
			http.Error(w, "failed to set customDomain to NS: "+err.Error(), http.StatusInternalServerError)
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

// empty from/toDomain == turkey provided (sub)domain
func setCustomDomain(fromDomain, toDomain string) error {

	//update ret config
	retCm, err := cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Get(context.Background(), "ret-config", metav1.GetOptions{})
	if err != nil {
		return err
	}
	ret_from := fromDomain
	if ret_from == "" {
		ret_from = "<SUB_DOMAIN>.<HUB_DOMAIN>"
	}
	ret_to := toDomain
	if ret_to == "" {
		ret_to = "<SUB_DOMAIN>.<HUB_DOMAIN>"
	}

	Logger.Sugar().Debugf("setCustomDomain, ret_from: %v, ret_to: %v", ret_from, ret_to)
	retCm.Data["config.toml.template"] =
		strings.Replace(
			retCm.Data["config.toml.template"], ret_from, ret_to, -1)
	_, err = cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Update(context.Background(), retCm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	//update hubs and spoke's env var
	hubs_from := fromDomain
	if hubs_from == "" {
		hubs_from = cfg.SubDomain + "." + cfg.HubDomain
	}
	hubs_to := toDomain
	if hubs_to == "" {
		hubs_to = cfg.SubDomain + "." + cfg.HubDomain
	}
	Logger.Sugar().Debugf("setCustomDomain, hubs_from: %v, hubs_to: %v", hubs_from, hubs_to)
	for _, appName := range []string{"hubs", "spoke"} {
		d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), appName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for i, env := range d.Spec.Template.Spec.Containers[0].Env {
			d.Spec.Template.Spec.Containers[0].Env[i].Value =
				strings.Replace(env.Value, hubs_from, hubs_to, -1)
		}
		_, err = cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	//add ingress route -- todo: replace it?
	err = ingress_addCustomDomainRule(hubs_to)
	if err != nil {
		return err
	}

	err = ingress_updateHaproxyCors("https://" + hubs_to)

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
		SecretName: "cert-" + customDomain,
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
