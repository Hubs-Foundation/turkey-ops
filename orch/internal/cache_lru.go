package internal

import (
	"errors"
	"reflect"
	"strconv"

	"github.com/golang/groupcache"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type GCache struct {
	Pool  *groupcache.HTTPPool
	Peers []string
	Group *groupcache.Group
}

var Cache *GCache

func StartLruCache() {
	pool := groupcache.NewHTTPPoolOpts("http://"+Cfg.PodIp+":"+Cfg.Port, &groupcache.HTTPPoolOptions{})

	Cache = &GCache{
		Pool: pool,
	}

	_, err := Cache.StartWatchingPeerPods()
	if err != nil {
		Logger.Error("failed to StartCache: " + err.Error())
	}
}

func (c *GCache) StartWatchingPeerPods() (chan struct{}, error) {
	if Cfg.K8ss_local == nil {
		return nil, errors.New("Cfg.K8ss_local == nil")
	}

	watchlist := cache.NewFilteredListWatchFromClient(
		Cfg.K8ss_local.ClientSet.CoreV1().RESTClient(),
		"pods",
		Cfg.PodNS,
		func(options *metav1.ListOptions) {
			options.LabelSelector = labels.SelectorFromSet(labels.Set(map[string]string{"app": Cfg.PodLabelApp})).String()
		},
	)

	_, controller := cache.NewInformer(
		watchlist,
		&v1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				// Logger.Sugar().Debugf("pod added: %s \n", obj)
				Logger.Sugar().Debugf("pod added")
				c.updatePeers(obj)
			},
			DeleteFunc: func(obj interface{}) {
				Logger.Sugar().Debugf("pod deleted")
				c.updatePeers(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				Logger.Sugar().Debugf("pod updated")
				c.updatePeers(newObj)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
	return stop, nil
}

func (c *GCache) updatePeers(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		Logger.Error("expected type v1.Endpoints but got:" + reflect.TypeOf(obj).String())
	}
	// Logger.Sugar().Debugf("updatePeers : %s \n", pod)
	Logger.Debug("updatePeers: pod.Status.PodIP == " + pod.Status.PodIP)
	for _, c := range pod.Status.ContainerStatuses {
		Logger.Debug("updatePeers: pod.Status.ContainerStatuses[0].Name == " + c.Name)
		Logger.Debug("updatePeers: pod.Status.ContainerStatuses[0].Ready == " + strconv.FormatBool(c.Ready))
	}

	// c.Pool.Set(peers...)

}
