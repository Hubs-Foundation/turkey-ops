package internal

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

var mu sync.Mutex

type TurkeyUpdater struct {
	containers            map[string]turkeyContainerInfo
	stopCh                chan struct{}
	channel               string
	publisherNS           string
	publisherCfgMapPrefix string //cfgMap's name will be prefix + channel, ie. hubsbuilds-beta
}
type turkeyContainerInfo struct {
	containerTag         string
	parentDeploymentName string
}

func NewTurkeyUpdater() *TurkeyUpdater {
	return &TurkeyUpdater{
		publisherNS:           "turkey-services",
		publisherCfgMapPrefix: "hubsbuilds-",
	}
}

func (u *TurkeyUpdater) loadContainers() error {

	u.containers = make(map[string]turkeyContainerInfo)

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
			u.containers[imgNameTagArr[0]] = turkeyContainerInfo{
				containerTag:         imgNameTagArr[1],
				parentDeploymentName: d.Name,
			}
		}
	}
	Logger.Sugar().Debugf("servicing ("+strconv.Itoa(len(u.containers))+") Containers (repo:{tag:parentDeployment}): %v", u.containers)
	return nil
}

func (u *TurkeyUpdater) Channel() string {
	return u.channel
}
func (u *TurkeyUpdater) Containers() string {
	return fmt.Sprint(u.containers)
}

func (u *TurkeyUpdater) Start(channel string) error {
	mu.Lock()
	defer mu.Unlock()

	err := u.loadContainers()
	if err != nil {
		return err
	}

	if u.stopCh != nil {
		close(u.stopCh)
		Logger.Info("restarting with channel: " + channel)
	} else {
		Logger.Info("starting with channel: " + channel)
	}

	if _, ok := cfg.SupportedChannels[channel]; !ok {
		Logger.Sugar().Warnf("bad channel %v, exiting without start", channel)
		return nil
	}

	u.stopCh, err = u.startWatchingPublisher(channel)
	if err != nil {
		Logger.Error("failed to startWatchingPublisher: " + err.Error())
	}
	return nil
}

func (u *TurkeyUpdater) startWatchingPublisher(channel string) (chan struct{}, error) {

	watchlist := cache.NewFilteredListWatchFromClient(
		cfg.K8sClientSet.CoreV1().RESTClient(),
		"configmaps",
		u.publisherNS,
		func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=" + u.publisherCfgMapPrefix + channel
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
				Logger.Sugar().Warnf("cfgmap label deleted ??? %s -- did someone do it manually?", obj)
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
	Logger.Sugar().Debugf("...received on <"+cfgmap.Name+"."+eventType+">: %v", cfgmap.Labels)
	Logger.Sugar().Debugf("...current u.containers : %v", u.containers)

	rand.Seed(int64(cfg.HostnameHash))
	waitSec := rand.Intn(300) + 30
	Logger.Sugar().Debugf("deployment starting in %v secs", waitSec)
	time.Sleep(time.Duration(waitSec) * time.Second) // so some namespaces will pull the new container images first and have them cached locally -- less likely for us to get rate limited

	for img, info := range u.containers {
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

			err := u.tryDeployNewContainer(img, newtag, info, 6)
			if err != nil {
				Logger.Error("deployNewContainer failed: " + err.Error())
				continue
			}
			u.containers[img] = turkeyContainerInfo{
				parentDeploymentName: u.containers[img].parentDeploymentName,
				containerTag:         newtag,
			}

		}
	}
}

func (u *TurkeyUpdater) tryDeployNewContainer(img, newtag string, info turkeyContainerInfo, maxRetry int) error {
	err := u.deployNewContainer(img, newtag, info)
	for err != nil && maxRetry > 0 {
		time.Sleep(10 * time.Second)
		err = u.deployNewContainer(img, newtag, info)
		maxRetry -= 1
	}
	return err
}

func (u *TurkeyUpdater) deployNewContainer(repo, newTag string, containerInfo turkeyContainerInfo) error {

	d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), containerInfo.parentDeploymentName, metav1.GetOptions{})
	if err != nil {
		Logger.Error("failed to get deployments <" + containerInfo.parentDeploymentName + "> in ns <" + cfg.PodNS + ">, err: " + err.Error())
		return err
	}

	err = k8s_waitForPods(d.Namespace, 180)
	if err != nil {
		return err
	}

	d, err = k8s_waitForDeployment(d, 180)
	if err != nil {
		return err
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
		time.Sleep(5 * time.Second)
		timeoutSec -= 5
		d, _ = cfg.K8sClientSet.AppsV1().Deployments(d.Namespace).Get(context.Background(), d.Name, metav1.GetOptions{})
		if timeoutSec < 1 {
			return d, errors.New("timeout while waiting for deployment <" + d.Name + ">")
		}
	}

	time.Sleep(5 * time.Second) // time for k8s master services to sync, should be more than enough, or we'll get pending pods stuck forever
	return d, nil
}

func k8s_waitForPods(namespace string, timeout int) error {
	timeoutSec := timeout
	pods, err := cfg.K8sClientSet.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		podStatusPhase := pod.Status.Phase
		for podStatusPhase == corev1.PodPending {
			Logger.Sugar().Debugf("waiting for pending pod %v / %v", pod.Namespace, pod.Name)
			time.Sleep(5 * time.Second)
			timeoutSec -= 5
			pod, err := cfg.K8sClientSet.CoreV1().Pods(namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
			podStatusPhase = pod.Status.Phase
			if err != nil {
				return err
			}
		}
		if timeoutSec < 1 {
			return errors.New("timeout while waiting for pod: " + pod.Name + " in ns: " + namespace)
		}
	}
	return nil
}
