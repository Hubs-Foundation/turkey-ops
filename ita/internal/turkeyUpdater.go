package internal

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"

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
	for k, v := range u.Containers {
		newtag, ok := cfgmap.Labels[k]
		if ok {
			if v.containerTag == newtag {
				Logger.Sugar().Info("NOT updating ... same tag: " + newtag)
				return
			}
			if v.containerTag > newtag {
				Logger.Warn("potential downgrade/rollback: new tag is lexicographically smaller than current")
			}
			Logger.Sugar().Info("updating " + k + ": " + v.containerTag + " --> " + newtag)
			err := u.deployNewContainer(k, newtag, v)
			if err != nil {
				Logger.Error("deployNewContainer failed: " + err.Error())
				return
			}
			// u.loadContainers()
			u.Containers[k] = turkeyContainerInfo{
				parentDeploymentName: u.Containers[k].parentDeploymentName,
				containerTag:         newtag,
			}
		} else {
			Logger.Debug("not found in cfgmap.Labels: " + k)
		}

	}
}

func (u *TurkeyUpdater) deployNewContainer(repo, newTag string, containerInfo turkeyContainerInfo) error {

	d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), containerInfo.parentDeploymentName, metav1.GetOptions{})
	if err != nil {
		Logger.Error("failed to get deployments <" + containerInfo.parentDeploymentName + "> in ns <" + cfg.PodNS + ">, err: " + err.Error())
		return err
	}
	for idx, c := range d.Spec.Template.Spec.Containers {
		imgNameTagArr := strings.Split(c.Image, ":")
		if imgNameTagArr[0] == repo {
			d.Spec.Template.Spec.Containers[idx].Image = repo + ":" + newTag
			_, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			return nil
		}
	}

	return errors.New("did not find repo name: " + repo + ", failed to deploy newTag: " + newTag)
}
