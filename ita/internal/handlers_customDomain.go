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
	features := cfg.Features.Get()
	if !features.customDomain || (r.URL.Path != "/custom-domain" && r.URL.Path != "/api/ita/custom-domain") {
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
		} else {
			fromDomain = cfg.SubDomain + "." + cfg.HubDomain
		}

		if toDomain != "" { //certbotbot
			letsencryptAcct := pickLetsencryptAccountForHubId()
			Logger.Sugar().Debugf("letsencryptAcct: %v", letsencryptAcct)
			err := runCertbotbotpod(letsencryptAcct, toDomain)
			if err != nil {
				http.Error(w, "failed @ runCertbotbotpod: "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			toDomain = cfg.SubDomain + "." + cfg.HubDomain
		}

		Logger.Sugar().Debugf("calling setCustomDomain with fromDomain: %v, toDomain: %v", fromDomain, toDomain)
		err := setCustomDomain(fromDomain, toDomain)
		if err != nil {
			http.Error(w, "failed @ setCustomDomain: "+err.Error(), http.StatusInternalServerError)
			return
		}

		err = ingress_addCustomDomainRule(fromDomain, toDomain)
		if err != nil {
			http.Error(w, "failed @ setCustomDomain: "+err.Error(), http.StatusInternalServerError)
			return
		}

		err = ingress_updateHaproxyCors(fromDomain, toDomain)
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
	features := cfg.Features.Get()
	if !features.customDomain || (r.URL.Path != "/update-cert" && r.URL.Path != "/api/ita/update-cert") {
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

// empty from/toDomain == turkey provided / native (sub)domain
func setCustomDomain(fromDomain, toDomain string) error {

	cfg.K8Man.WorkBegin("setCustomDomain")
	defer cfg.K8Man.WorkEnd("setCustomDomain")

	//update ret config
	retCm, err := cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Get(context.Background(), "ret-config", metav1.GetOptions{})
	if err != nil {
		return err
	}
	ret_from := fromDomain
	if ret_from == cfg.SubDomain+"."+cfg.HubDomain {
		ret_from = "<SUB_DOMAIN>.<HUB_DOMAIN>"
	}
	ret_to := toDomain
	if ret_to == cfg.SubDomain+"."+cfg.HubDomain {
		ret_to = "<SUB_DOMAIN>.<HUB_DOMAIN>"
	}

	Logger.Sugar().Debugf("setCustomDomain, ret_from: %v, ret_to: %v", ret_from, ret_to)
	retCm.Data["config.toml.template"] =
		strings.Replace(
			retCm.Data["config.toml.template"], `host = "`+ret_from, `host = "`+ret_to, -1)
	retCm.Data["config.toml.template"] =
		strings.Replace(
			retCm.Data["config.toml.template"], `host = "https://`+ret_from, `host = "https://`+ret_to, -1)
	retCm.Data["config.toml.template"] =
		strings.Replace(
			retCm.Data["config.toml.template"], `issuer = "`+ret_from, `issuer = "`+ret_to, -1)
	_, err = cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Update(context.Background(), retCm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	//update hubs and spoke's env var
	Logger.Sugar().Debugf("setCustomDomain, hubs_from: %v, hubs_to: %v", fromDomain, toDomain)
	for _, appName := range []string{"hubs", "spoke"} {
		d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), appName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for i, env := range d.Spec.Template.Spec.Containers[0].Env {
			d.Spec.Template.Spec.Containers[0].Env[i].Value =
				strings.Replace(env.Value, fromDomain, toDomain, -1)
		}
		_, err = cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func ingress_addCustomDomainRule(customDomain, fromDomain string) error {

	cfg.K8Man.WorkBegin("ingress_addCustomDomainRule")
	defer cfg.K8Man.WorkEnd("ingress_addCustomDomainRule")

	igs, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	ig, retRootRules, err := findIngressWithRetRootRules(&igs.Items)
	if err != nil {
		Logger.Error("findIngressWithRetRootPath failed: " + err.Error())
		return err
	}
	Logger.Sugar().Debugf("retRootRules: %v", retRootRules)

	if fromDomain != cfg.SubDomain+"."+cfg.HubDomain {
		// fromDomainSecretName := "cert-" + fromDomain
		// err := cfg.K8sClientSet.CoreV1().Secrets(cfg.PodNS).Delete(context.Background(), fromDomainSecretName, metav1.DeleteOptions{})
		// if err != nil {
		// 	Logger.Sugar().Warnf("failed to delete fromDomain's cert: %v, err: %v", fromDomainSecretName, err)
		// }
		deletedRules, deletedTlss := ingress_cleanupByDomain(ig, fromDomain)
		if deletedRules+deletedTlss > 0 {
			ig.ResourceVersion = ""
			cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Update(context.Background(), ig, metav1.UpdateOptions{})
		}
	}

	if b, rule := ingressRuleAlreadyCreated_byBackendHost(ig, customDomain); b { // ingressRuleAlreadyCreated
		Logger.Sugar().Warnf("this is unexpected -- ingressRuleAlreadyCreated_byBackendHost: %v", rule)
		return nil
	}

	customDomainRule := retRootRules[0].DeepCopy()
	customDomainRule.Host = customDomain
	ig.Spec.Rules = append(ig.Spec.Rules, *customDomainRule)

	ig.Spec.TLS = append(ig.Spec.TLS, networkingv1.IngressTLS{
		Hosts:      []string{customDomain},
		SecretName: "cert-" + customDomain,
	})
	ig.ResourceVersion = ""
	newIg, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Update(context.Background(), ig, metav1.UpdateOptions{})
	if err != nil {
		Logger.Sugar().Errorf("failed to update ingress with customDomainRule: %v", err)
		return err
	}
	Logger.Sugar().Debugf("updated ingress: %v", newIg)

	return nil
}

func ingress_updateHaproxyCors(from, to string) error {

	cfg.K8Man.WorkBegin("ingress_updateHaproxyCors")
	defer cfg.K8Man.WorkEnd("ingress_updateHaproxyCors")

	igs, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, ig := range igs.Items {
		ig.Annotations["haproxy.org/response-set-header"] = strings.Replace(ig.Annotations["haproxy.org/response-set-header"], from, to, -1)
		_, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Update(context.Background(), &ig, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func ingress_cleanupByDomain(ig *networkingv1.Ingress, domain string) (int, int) {

	trimmedRules := []networkingv1.IngressRule{}
	trimmedTlss := []networkingv1.IngressTLS{}

	deletedRules := 0
	deleteTlss := 0

	for _, rule := range ig.Spec.Rules {
		if rule.Host == domain {
			Logger.Sugar().Debugf("dropping rule: %v", rule)
			deletedRules++
			continue
		}
		trimmedRules = append(trimmedRules, rule)
	}
	for _, tls := range ig.Spec.TLS {
		add := true
		for _, host := range tls.Hosts {
			if host == domain {
				Logger.Sugar().Debugf("dropping tls: %v", tls)
				add = false
				deleteTlss++
				break
			}
		}
		if add {
			trimmedTlss = append(trimmedTlss, tls)
		}
	}

	ig.Spec.Rules = trimmedRules
	ig.Spec.TLS = trimmedTlss

	Logger.Sugar().Debugf("deletedRules: %v, deletedTlss: %v", deletedRules, deleteTlss)

	return deletedRules, deleteTlss
}
