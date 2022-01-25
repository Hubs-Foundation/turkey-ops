package internal

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type TurkeyUpdater struct {
	Channel    string
	Containers map[string]string
}

func NewTurkeyUpdater(channel string) *TurkeyUpdater {
	return &TurkeyUpdater{
		Channel: channel,
	}
}

func (u *TurkeyUpdater) ReLoad() error {

	u.Containers = make(map[string]string)

	dList, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).List(context.Background(), apiv1.ListOptions{})
	if err != nil {
		Logger.Error("failed to list deployments for ns: " + cfg.PodNS + ", err: " + err.Error())
		return err
	}

	for _, d := range dList.Items {
		for _, c := range d.Spec.Template.Spec.Containers {
			imgNameTagArr := strings.Split(c.Image, ":")
			u.Containers[imgNameTagArr[0]] = imgNameTagArr[1]
		}
	}

	Logger.Sugar().Debugf("u.Containers: %v", u.Containers)

	return nil
}

func (u *TurkeyUpdater) Start(publisherNsName string) error {

	err := u.ReLoad()
	if err != nil {
		return err
	}

	return nil
}

func (u *TurkeyUpdater) startWatchingPublisherNs(publisherNsName string) (chan struct{}, error) {

	watchlist := cache.NewFilteredListWatchFromClient(
		cfg.K8sClientSet.CoreV1().RESTClient(),
		"namespace",
		cfg.PodNS,
		func(options *metav1.ListOptions) {},
	)

	_, controller := cache.NewInformer(
		watchlist,
		&corev1.Namespace{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				// logger.Sugar().Debugf("pod added: %s \n", obj)
				Logger.Sugar().Debugf("added")
				// c.updatePeers(obj)
			},
			DeleteFunc: func(obj interface{}) {
				Logger.Sugar().Debugf("deleted")
				// c.updatePeers(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				Logger.Sugar().Debugf("updated")
				// c.updatePeers(newObj)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
	return stop, nil

}
