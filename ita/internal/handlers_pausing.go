package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var Root_Pausing = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		bytes, err := ioutil.ReadFile("./_statics/pausing.html")
		if err != nil {
			Logger.Sugar().Errorf("%v", err)
		}
		fmt.Fprint(w, string(bytes))
	}

})

var Z_Pause = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	err := HC_Pause()

	fmt.Fprintf(w, "err: %v", err)
})

var Z_Resume = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		err := HC_Resume()

		if err != nil {
			Logger.Sugar().Errorf("err (reqId: %v): %v", w.Header().Get("X-Request-Id"), err)
			fmt.Fprintf(w, "something went wrong -- take this to support: reqId=%v", w.Header().Get("X-Request-Id"))
		}

		Logger.Sugar().Debugf("_resuming_status: %v", _resuming_status)

		if _resuming_status == 0 {
			fmt.Fprint(w, "this hubs' paused, click the duck to try to unpause it")
			return
		}
		if _resuming_status < 0 {
			fmt.Fprintf(w, "resuming, this can take a few minutes")
			return
		}
		fmt.Fprintf(w, "not ready yet, try again in %v min", (_resuming_status / 60))
	}
})

func HC_Pause() error {

	//back up current ingresses
	igs, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	igsbak, err := json.Marshal(*igs)
	if err != nil {
		Logger.Error("failed to marshal ingresses: " + err.Error())
	}

	igsbak_cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "igsbak"},
		BinaryData: map[string][]byte{"igsbak": igsbak},
	}
	_, err = cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Create(context.Background(), igsbak_cm, metav1.CreateOptions{})
	if err != nil {
		_, err = cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Update(context.Background(), igsbak_cm, metav1.UpdateOptions{})
		if err != nil {
			Logger.Error("failed to create/update ig_bak configmap:" + err.Error())
		}
	}

	//delete current ingresses
	for _, ig := range igs.Items {
		err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Delete(context.Background(), ig.Name, metav1.DeleteOptions{})
		if err != nil {
			Logger.Error("failed to delete ingresses" + err.Error())
		}
	}
	//create pausing ingress
	pathType_exact := networkingv1.PathTypeExact
	pathType_prefix := networkingv1.PathTypePrefix
	_, err = cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Create(context.Background(),
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "pausing",
				Annotations: map[string]string{"kubernetes.io/ingress.class": "haproxy"},
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						Host: cfg.SubDomain + "." + cfg.HubDomain,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathType_exact,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "ita",
												Port: networkingv1.ServiceBackendPort{
													Number: 6000,
												}}}},
									{
										Path:     "/z/resume",
										PathType: &pathType_prefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "ita",
												Port: networkingv1.ServiceBackendPort{
													Number: 6000,
												}}}},
								}}}}}},
		}, metav1.CreateOptions{})
	if err != nil {
		Logger.Error("failed to create pausing ingresses" + err.Error())
	}

	// scale down deployments, except ita
	ds, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error("failed to list deployments: " + err.Error())
		return err
	}
	for _, d := range ds.Items {
		if d.Name == "ita" {
			continue
		}
		d.Spec.Replicas = pointerOfInt32(0)
		_, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), &d, metav1.UpdateOptions{})
		if err != nil {
			Logger.Sugar().Errorf("failed to scale down %v: %v"+d.Name, err.Error())
			return err
		}
	}

	return nil
}

var _resuming_status = int32(0)

func HC_Resume() error {
	Logger.Sugar().Debugf("_resuming_status=%v", _resuming_status)
	if _resuming_status != 0 {
		return nil
	}
	Logger.Sugar().Debugf("resuming in progress, _resuming_status=%v", _resuming_status)
	atomic.StoreInt32(&_resuming_status, -1)
	// scale back deployments
	ds, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error("failed to list deployments: " + err.Error())
		return err
	}
	for _, d := range ds.Items {
		if d.Name == "ita" {
			continue
		}
		d.Spec.Replicas = pointerOfInt32(1)
		_, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), &d, metav1.UpdateOptions{})
		if err != nil {
			Logger.Sugar().Errorf("failed to scale back %v: %v"+d.Name, err.Error())
			return err
		}
	}
	go func() {
		//wait for ret pod
		ret_readyReplicaCnt := 0
		ttl := 5 * time.Minute
		for ret_readyReplicaCnt < 1 && ttl > 0 {
			ret_d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), "reticulum", metav1.GetOptions{})
			if err != nil {
				Logger.Sugar().Errorf("failed to get reticulum deployment in ns %v", cfg.PodNS)
				time.Sleep(5 * time.Second)
				continue
			}
			ret_readyReplicaCnt = int(ret_d.Status.ReadyReplicas)
			Logger.Sugar().Debugf("waiting for ret, ttl: %v", ttl)
			time.Sleep(30 * time.Second)
			ttl -= 30 * time.Second
		}
		Logger.Debug("ret's ready")

		// delete pausing ingress
		err = cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Delete(context.Background(), "pausing", metav1.DeleteOptions{})
		if err != nil {
			Logger.Error("failed to delete pausing ingresses" + err.Error())
		}

		//restore ig_bak
		igsbak_cm, err := cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Get(context.Background(), "igsbak", metav1.GetOptions{})
		if err != nil {
			Logger.Error("failed to get ig_bak configmap:" + err.Error())
		}
		igsbak := igsbak_cm.BinaryData["igsbak"]
		var igs networkingv1.IngressList
		err = json.Unmarshal(igsbak, &igs)
		if err != nil {
			Logger.Sugar().Errorf("failed to unmarshal igsbak: %v", err)
		}
		for _, ig := range igs.Items {
			ig.ResourceVersion = ""
			_, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Create(context.Background(), &ig, metav1.CreateOptions{})
			if err != nil {
				Logger.Sugar().Errorf("failed to restore ig_bak: %v", err)
			}
		}

		cooldown := 3600
		for cooldown > 0 {
			time.Sleep(11 * time.Second)
			cooldown -= 11
			atomic.StoreInt32(&_resuming_status, int32(cooldown))
		}
		atomic.StoreInt32(&_resuming_status, 0)
	}()

	return nil
}
