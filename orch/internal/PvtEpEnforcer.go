package internal

import (
	"net"
	"net/http"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/strings/slices"
)

type PvtEpEnforcer struct {
	epWatchList []string
	epData      map[string][]string
}

type ipData struct {
	ShouldAllow bool
	BestBy      time.Time
}

func NewPvtEpEnforcer(epWatchList []string) *PvtEpEnforcer {
	return &PvtEpEnforcer{
		epWatchList: epWatchList,
		epData:      make(map[string][]string),
	}
}

func (p *PvtEpEnforcer) Filter(allowedKubeSvcs []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sourceIp := strings.Split(r.RemoteAddr, ":")[0]
			Logger.Debug("accessed from: " + sourceIp)

			if p.shoudAllow(sourceIp, allowedKubeSvcs) {
				next.ServeHTTP(w, r)
				return
			}
			Logger.Sugar().Debugf("NOT allowed: %v", sourceIp)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		})
	}
}

func (p *PvtEpEnforcer) shoudAllow(ip string, allowedKubeSvcs []string) bool {

	if !net.ParseIP(ip).IsPrivate() {
		Logger.Warn("!!! private endpoint accessed by non-private-ip:" + ip)
		return false
	}
	// * == allow all internal ips
	if slices.Contains(allowedKubeSvcs, "*") {
		Logger.Sugar().Debugf("allowed: %v", ip)
		return true
	}
	//
	for _, allowedKubeSvc := range allowedKubeSvcs {
		svcData, ok := p.epData[allowedKubeSvc]
		if ok && slices.Contains(svcData, ip) {
			Logger.Sugar().Debugf("allowed: [%v]: %v", allowedKubeSvc, ip)
			return true
		}
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
			&corev1.Endpoints{},
			0,
			cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					ep := obj.(*corev1.Endpoints)
					Logger.Sugar().Debugf("added: %v", ep)
					p.refreshEpData(ep.Name+"."+ep.Namespace, ep)
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					ep := newObj.(*corev1.Endpoints)
					Logger.Sugar().Debugf("updated: %v", ep)
					p.refreshEpData(ep.Name+"."+ep.Namespace, ep)
				},
				DeleteFunc: func(obj interface{}) {
					ep := obj.(*corev1.Endpoints)
					Logger.Sugar().Debugf("deleted: %v", ep)
					delete(p.epData, ep.Name+"."+ep.Namespace)
				},
			},
		)
		go controller.Run(make(chan struct{}))
	}
	return nil
}

func (p *PvtEpEnforcer) refreshEpData(allowedKubeSvc string, ep *corev1.Endpoints) {
	ips := []string{}
	for _, sub := range ep.Subsets {
		for _, addr := range sub.Addresses {
			ips = append(ips, addr.IP)
		}
	}
	p.epData[allowedKubeSvc] = ips
	Logger.Sugar().Debugf("refreshed: epData[%v]=%v", allowedKubeSvc, ips)
}
