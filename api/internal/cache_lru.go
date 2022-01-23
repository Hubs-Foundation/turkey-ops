package internal

import (
	"errors"
	"reflect"

	"github.com/golang/groupcache"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

type GCache struct {
	Pool  *groupcache.HTTPPool
	Group *groupcache.Group
}

var Cache *GCache

func StartLruCache() {
	pool := groupcache.NewHTTPPoolOpts("http://"+Cfg.PodIp+":"+Cfg.Port, &groupcache.HTTPPoolOptions{})

	Cache = &GCache{
		Pool: pool,
	}

	_, err := Cache.StartWatchingPods()
	if err != nil {
		logger.Error("failed to StartCache: " + err.Error())
	}
}

func (c *GCache) StartWatchingPods() (chan struct{}, error) {
	if Cfg.K8ss_local == nil {
		return nil, errors.New("Cfg.K8ss_local == nil")
	}

	watchlist := cache.NewListWatchFromClient(
		Cfg.K8ss_local.ClientSet.CoreV1().RESTClient(),
		"pods",
		Cfg.PodNS,
		fields.OneTermEqualSelector("app", "turkey-api"),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&v1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				logger.Sugar().Debugf("pod added: %s \n", obj)
				c.updatePeers(obj)
			},
			DeleteFunc: func(obj interface{}) {
				logger.Sugar().Debugf("pod deleted: %s \n", obj)
				c.updatePeers(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				logger.Sugar().Debugf("pod changed from %s to %s\n", oldObj, newObj)
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
		logger.Error("expected type v1.Endpoints but got:" + reflect.TypeOf(obj).String())
	}
	logger.Sugar().Debugf("updatePeers : %s \n", pod)
	// c.Pool.Set(peers...)
}
