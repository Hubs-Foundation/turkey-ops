package internal

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type TurkeyUpdater struct {
	Channel    string
	Containers map[string]turkeyContainerInfo
	stopCh     chan struct{}

	publisherNS           string
	publisherCfgMapPrefix string //cfgMap's name will be prefix + channel, ie. hubsbuilds-beta
}
type turkeyContainerInfo struct {
	containerTag         string
	parentDeploymentName string
}

func NewTurkeyUpdater() *TurkeyUpdater {
	return &TurkeyUpdater{
		Channel:               cfg.ListeningChannel,
		publisherNS:           "turkey-services",
		publisherCfgMapPrefix: "hubsbuilds-",
	}
}

func (u *TurkeyUpdater) loadContainers() error {

	u.Containers = make(map[string]turkeyContainerInfo)

	dList, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error("failed to list deployments for ns: " + cfg.PodNS + ", err: " + err.Error())
		return err
	}
	Logger.Sugar().Debugf("deployments in local namespace("+cfg.PodNS+"): %v", len(dList.Items))

	for _, d := range dList.Items {
		for _, c := range d.Spec.Template.Spec.Containers {
			imgNameTagArr := strings.Split(c.Image, ":")
			if len(imgNameTagArr) < 2 {
				return errors.New("problem -- bad Image Name: " + c.Image)
			}
			u.Containers[imgNameTagArr[0]] = turkeyContainerInfo{
				containerTag:         imgNameTagArr[1],
				parentDeploymentName: d.Name,
			}
		}
	}
	Logger.Sugar().Debugf("servicing ("+strconv.Itoa(len(u.Containers))+") Containers (repo:{tag:parentDeployment}): %v", u.Containers)
	return nil
}

func (u *TurkeyUpdater) Start() (chan struct{}, error) {
	if u.stopCh != nil {
		close(u.stopCh)
		Logger.Info("restarting for channel: " + u.Channel)
	} else {
		Logger.Info("starting for channel: " + u.Channel)
	}

	err := u.loadContainers()
	if err != nil {
		return nil, err
	}

	stop, err := u.startWatchingPublisher()
	if err != nil {
		Logger.Error("failed to startWatchingPublisher: " + err.Error())
	}
	return stop, nil
}

func (u *TurkeyUpdater) startWatchingPublisher() (chan struct{}, error) {

	watchlist := cache.NewFilteredListWatchFromClient(
		cfg.K8sClientSet.CoreV1().RESTClient(),
		"configmaps",
		u.publisherNS,
		func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=" + u.publisherCfgMapPrefix + u.Channel
		},
	)

	_, controller := cache.NewInformer(
		watchlist,
		&corev1.ConfigMap{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				u.handleEvents(obj, "add")
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				u.handleEvents(newObj, "update")
			},
			DeleteFunc: func(obj interface{}) {
				Logger.Sugar().Warnf("cfgmap label deleted ??? %s", obj)
				// u.handleEvents(obj)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
	return stop, nil
}

func (u *TurkeyUpdater) handleEvents(obj interface{}, eventType string) {
	cfgmap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		Logger.Error("expected type corev1.Namespace but got:" + reflect.TypeOf(obj).String())
	}
	Logger.Sugar().Debugf("received on <"+cfgmap.Name+"."+eventType+">, configmap.labels : %v", cfgmap.Labels)

	for img, info := range u.Containers {
		newtag, ok := cfgmap.Labels[img]
		if ok {
			if info.containerTag == newtag {
				Logger.Sugar().Info("NOT updating " + img + "... same tag: " + newtag)
				continue
			}
			if info.containerTag > newtag {
				Logger.Warn("potential downgrade/rollback: new tag is lexicographically smaller than current")
			}
			Logger.Sugar().Info("updating " + img + ": " + info.containerTag + " --> " + newtag)

			err := u.deployNewContainer(img, newtag, info)
			if err != nil {
				Logger.Error("deployNewContainer failed: " + err.Error())
				continue
			}
			u.Containers[img] = turkeyContainerInfo{
				parentDeploymentName: u.Containers[img].parentDeploymentName,
				containerTag:         newtag,
			}

		}
	}
}

func (u *TurkeyUpdater) deployNewContainer(repo, newTag string, containerInfo turkeyContainerInfo) error {

	d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), containerInfo.parentDeploymentName, metav1.GetOptions{})
	if err != nil {
		Logger.Error("failed to get deployments <" + containerInfo.parentDeploymentName + "> in ns <" + cfg.PodNS + ">, err: " + err.Error())
		return err
	}
	d, err = k8s_waitForDeployment(d, 300)
	if err != nil {

	}
	for idx, c := range d.Spec.Template.Spec.Containers {
		imgNameTagArr := strings.Split(c.Image, ":")
		if imgNameTagArr[0] == repo {
			d.Spec.Template.Spec.Containers[idx].Image = repo + ":" + newTag
			d_new, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			Logger.Sugar().Debugf("d_new.Spec.Template.Spec.Containers[0]: %v", d_new.Spec.Template.Spec.Containers[0])
			return nil
		}
	}
	return errors.New("did not find repo name: " + repo + ", failed to deploy newTag: " + newTag)
}

func k8s_waitForDeployment(d *appsv1.Deployment, timeout int) (*appsv1.Deployment, error) {
	timeoutSec := timeout
	for d.Status.Replicas != d.Status.AvailableReplicas ||
		d.Status.Replicas != d.Status.ReadyReplicas ||
		d.Status.Replicas != d.Status.UpdatedReplicas {
		Logger.Sugar().Debugf("waiting for %v -- currently: Replicas=%v, Available=%v, Ready=%v, Updated=%v",
			d.Name, d.Status.Replicas, d.Status.AvailableReplicas, d.Status.ReadyReplicas, d.Status.UpdatedReplicas)
		time.Sleep(3 * time.Second)
		timeoutSec -= 3
		d, _ = cfg.K8sClientSet.AppsV1().Deployments(d.Namespace).Get(context.Background(), d.Name, metav1.GetOptions{})
		if timeoutSec < 1 {
			return d, errors.New("timeout while waiting for deployment <" + d.Name + ">")
		}
	}
	return d, nil
}
