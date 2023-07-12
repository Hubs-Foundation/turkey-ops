package internal

import (
	"errors"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type trcCmBook struct {
	book     map[string]string
	lastUsed map[string]time.Time
	mu       sync.Mutex
}

func NewTrcCmBook() *trcCmBook {
	b := &trcCmBook{
		book:     map[string]string{},
		lastUsed: map[string]time.Time{},
	}
	// b.startWatching()
	return b
}

func (tcb *trcCmBook) GetHubId(subdomain string) string {
	tcb.mu.Lock()
	defer tcb.mu.Unlock()
	return tcb.book[subdomain]
}

func (tcb *trcCmBook) RecUsage(subdomain string) {
	tcb.mu.Lock()
	defer tcb.mu.Unlock()
	tcb.lastUsed[subdomain] = time.Now()
}
func (tcb *trcCmBook) GetLastUsed(subdomain string) time.Time {
	tcb.mu.Lock()
	defer tcb.mu.Unlock()
	return tcb.lastUsed[subdomain]
}

func (tcb *trcCmBook) set(newBook map[string]string) {
	tcb.mu.Lock()
	defer tcb.mu.Unlock()
	tcb.book = newBook
}

func (tcb *trcCmBook) StartWatching() (chan struct{}, error) {
	if Cfg.K8ss_local.ClientSet == nil {
		return nil, errors.New("k8.ClientSet == nil")
	}
	watchlist := cache.NewFilteredListWatchFromClient(
		Cfg.K8ss_local.ClientSet.CoreV1().RESTClient(),
		"configmaps",
		Cfg.PodNS,
		func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=turkey-return-center"
		},
	)
	_, controller := cache.NewInformer(
		watchlist,
		&corev1.ConfigMap{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				GetLogger().Sugar().Debugf("added: %v", obj)
				cm := obj.(*corev1.ConfigMap)
				tcb.set(cm.Data)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				GetLogger().Sugar().Debugf("updated: %v", newObj)
				cm := newObj.(*corev1.ConfigMap)
				tcb.set(cm.Data)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
	return stop, nil
}
