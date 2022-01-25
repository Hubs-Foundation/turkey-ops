package internal

import (
	"context"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
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

	dList, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error("failed to list deployments for ns: " + cfg.PodNS + ", err: " + err.Error())
		return err
	}
	Logger.Sugar().Debugf("deployments: %v", len(dList.Items))

	for _, d := range dList.Items {
		for _, c := range d.Spec.Template.Spec.Containers {
			imgNameTagArr := strings.Split(c.Image, ":")
			u.Containers[imgNameTagArr[0]] = imgNameTagArr[1]
		}
	}
	Logger.Sugar().Debugf("containers: %v", len(u.Containers))

	Logger.Sugar().Debugf("u.Containers: %v", u.Containers)

	return nil
}

func (u *TurkeyUpdater) Start(publisherNsName string) (chan struct{}, error) {

	err := u.ReLoad()
	if err != nil {
		return nil, err
	}

	stop, err := u.startWatchingPublisher(publisherNsName)
	if err != nil {
		Logger.Error("failed to startWatchingPublisherNs: " + err.Error())
	}

	return stop, nil
}

func (u *TurkeyUpdater) startWatchingPublisher(publisherNsName string) (chan struct{}, error) {

	watchlist := cache.NewFilteredListWatchFromClient(
		cfg.K8sClientSet.CoreV1().RESTClient(),
		"configmaps",
		publisherNsName,
		func(options *metav1.ListOptions) { options.FieldSelector = "metadata.name=hubsbuilds" },
	)

	_, controller := cache.NewInformer(
		watchlist,
		&corev1.ConfigMap{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				// logger.Sugar().Debugf("pod added: %s \n", obj)
				Logger.Sugar().Debugf("added")
				u.doStuff(obj)
			},
			DeleteFunc: func(obj interface{}) {
				Logger.Sugar().Debugf("deleted")
				u.doStuff(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				Logger.Sugar().Debugf("updated")
				u.doStuff(newObj)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
	return stop, nil

}

func (u *TurkeyUpdater) doStuff(obj interface{}) {
	res, ok := obj.(*corev1.ConfigMap)
	if !ok {
		Logger.Error("expected type corev1.Namespace but got:" + reflect.TypeOf(obj).String())
	}
	Logger.Sugar().Debugf("received : %v", res.Data)

	// c.Pool.Set(peers...)

}
