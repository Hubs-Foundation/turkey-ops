package internal

import (
	"net"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/strings/slices"
)

type PvtEpEnforcer struct {
	epWatchList []string
	epData      map[string][]string
}

func NewPvtEpEnforcer(epWatchList []string) *PvtEpEnforcer {
	return &PvtEpEnforcer{
		epWatchList: epWatchList,
		epData:      make(map[string][]string),
	}
}

func (p *PvtEpEnforcer) Filter(allowedKubeSvc string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sourceIp := r.RemoteAddr
			Logger.Debug("accessed from: " + sourceIp)
			if p.shoudAllow(sourceIp, allowedKubeSvc) {
				next.ServeHTTP(w, r)
			}
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		})
	}
}

func (p *PvtEpEnforcer) shoudAllow(ip, allowedKubeSvc string) bool {
	if !net.ParseIP(ip).IsPrivate() {
		GetLogger().Warn("!!! private endpoint accessed by non-private-ip:" + ip)
		return false
	}
	if allowedKubeSvc == "*" {
		return true
	}

	svcData, ok := p.epData[allowedKubeSvc]
	if ok && slices.Contains(svcData, ip) {
		return true
	}

	return false
}

func (p *PvtEpEnforcer) StartWatching() error {
	for _, v := range p.epWatchList {
		v_arr := strings.Split(v, ".")
		if len(v_arr) != 2 {
			Logger.Sugar().Errorf("skipping bad epWatchList item: %v (expected: <svc_name>.<ns_name>)", v)
			continue
		}
		Logger.Info("watching: " + v)
		nsName := v_arr[1]
		svcName := v_arr[0]
		watchlist := cache.NewFilteredListWatchFromClient(
			Cfg.K8ss_local.ClientSet.CoreV1().RESTClient(),
			"endpoints",
			nsName,
			func(options *metav1.ListOptions) {
				options.FieldSelector = "metadata.name=" + svcName
			},
		)
		_, controller := cache.NewInformer(
			watchlist,
			&corev1.Namespace{},
			0,
			cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					GetLogger().Sugar().Debugf("added: %v", obj)
					// ns := obj.(*corev1.Namespace)
					// HC_NS_MAN.Set(ns.Name, HcNsNotes{Labels: ns.Labels, Lastchecked: time.Now()})
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					GetLogger().Sugar().Debugf("updated: %v", newObj)
					// ns := newObj.(*corev1.Namespace)
					// if ns.Annotations["deleting"] == "true" {
					// 	HC_NS_MAN.Del(ns.Name)
					// 	return
					// }
					// HC_NS_MAN.Set(ns.Name, HcNsNotes{Labels: ns.Labels, Lastchecked: time.Now()})
				},
				DeleteFunc: func(obj interface{}) {
					GetLogger().Sugar().Debugf("deleted: %v", obj)
					// ns := obj.(*corev1.Namespace)
					// HC_NS_MAN.Del(ns.Name)
				},
			},
		)
		go controller.Run(make(chan struct{}))
	}
	return nil
}
