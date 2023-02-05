package internal

import (
	"context"
	"fmt"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var CustomDomain = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/custom-domain" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	customDomain := r.Header.Get("custom-domain")

	letsencryptAcct := pickLetsencryptAccountForHubId()
	Logger.Sugar().Debugf("letsencryptAcct: %v", letsencryptAcct)

	cfg.K8sClientSet.CoreV1().Pods(cfg.PodNS).Create(
		context.Background(),
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("certbotbot-%v", time.Now().Unix()),
				Namespace: cfg.PodNS,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "certbotbot",
						Image: "mozillareality/certbotbot-http",
						Env: []corev1.EnvVar{
							{Name: "DOMAIN", Value: customDomain},
							{Name: "NAMESPACE", Value: cfg.PodNS},
							{Name: "LETSENCRYPT_ACCOUNT", Value: letsencryptAcct},
						},
					},
				},
			},
		},
		metav1.CreateOptions{},
	)

	// err := k8s_removeNfsMount("hubs")
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "done")
})
