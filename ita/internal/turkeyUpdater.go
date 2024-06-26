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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/strings/slices"
)

var mu_updater sync.Mutex

type TurkeyUpdater struct {
	// containers            map[string]turkeyContainerInfo
	containers            []turkeyContainerInfo
	stopCh                chan struct{}
	channel               string
	publisherNS           string
	publisherCfgMapPrefix string //cfgMap's name will be prefix + channel, ie. hubsbuilds-beta
}
type turkeyContainerInfo struct {
	containerRepo        string
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
			u.containers = append(u.containers, turkeyContainerInfo{
				containerRepo:        imgNameTagArr[0],
				containerTag:         imgNameTagArr[1],
				parentDeploymentName: d.Name,
			})
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

func (u *TurkeyUpdater) Start() error {
	mu_updater.Lock()
	defer mu_updater.Unlock()

	channel, err := Deployment_getLabel("CHANNEL")
	if err != nil {
		Logger.Warn("failed to get channel: " + err.Error())
		return err
	}

	err = u.loadContainers()
	if err != nil {
		return err
	}

	if u.stopCh != nil {
		close(u.stopCh)
		Logger.Info("restarting for channel: " + channel)
	} else {
		Logger.Info("starting for channel: " + channel)
	}

	if !slices.Contains(cfg.SupportedChannels, channel) {
		Logger.Sugar().Warnf("unexpected channel: %v, TurkeyUpdater will not start", channel)
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
	ts := time.Now().Unix()
	Logger.Sugar().Infof("started: %v", ts)

	Logger.Sugar().Debugf("...received on <"+cfgmap.Name+"."+eventType+">: %v", cfgmap.Labels)
	Logger.Sugar().Debugf("...current u.containers : %v", u.containers)

	if u.channel == "stable" {
		// so some namespaces will pull the new container images first
		// and have them cached locally -- less likely for us to get rate limited
		rand.Seed(int64(cfg.HostnameHash))
		waitSec := rand.Intn(1800) + 30
		Logger.Sugar().Debugf("stable channel: deployment will start in %v secs", waitSec)
		time.Sleep(time.Duration(waitSec) * time.Second)
	}

	for i, info := range u.containers {
		img := info.containerRepo
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

			err := u.tryDeployNewContainer(img, newtag, info, 3)
			if err != nil {
				Logger.Error("tryDeployNewContainer failed: " + err.Error())
			}
			u.containers[i] = turkeyContainerInfo{
				containerRepo:        img,
				parentDeploymentName: u.containers[i].parentDeploymentName,
				containerTag:         newtag,
			}
		}
	}

	Logger.Sugar().Infof("done: %v", ts)
}

func (u *TurkeyUpdater) tryDeployNewContainer(img, newtag string, info turkeyContainerInfo, maxRetry int) error {
	err := u.deployNewContainer(img, newtag, info, maxRetry)
	for err != nil && maxRetry > 0 {
		err = u.deployNewContainer(img, newtag, info, maxRetry)
		maxRetry -= 1
		time.Sleep(1 * time.Minute)
	}
	return err
}

func (u *TurkeyUpdater) deployNewContainer(repo, newTag string, containerInfo turkeyContainerInfo, retriesRemaining int) error {

	cfg.K8Man.WorkBegin(fmt.Sprintf("deployNewContainer for repo %v, newTagL %v, containerInfo: %v, retriesRemaining: %v", repo, newTag, containerInfo, retriesRemaining))
	defer cfg.K8Man.WorkEnd("deployNewContainer")

	d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), containerInfo.parentDeploymentName, metav1.GetOptions{})
	if err != nil {
		Logger.Error("failed to get deployments <" + containerInfo.parentDeploymentName + "> in ns <" + cfg.PodNS + ">, err: " + err.Error())
		return err
	}

	if d.Labels["ita_noUpdate"] != "" {
		Logger.Sugar().Debugf("skipping deployment %v -- (ita_noUpdate in label)", d.Name)
		return nil
	}

	if retriesRemaining > 1 {
		pods, err := cfg.K8sClientSet.CoreV1().Pods(d.Namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		err = k8s_waitForPods(pods, 2*time.Minute)
		if err != nil {
			return err
		}

		d, err = k8s_waitForDeployment(d, 2*time.Minute)
		if err != nil {
			return err
		}
	} else {
		Logger.Warn("last retry, skip waitings")
	}

	for idx, c := range d.Spec.Template.Spec.Containers {
		imgNameTagArr := strings.Split(c.Image, ":")
		if imgNameTagArr[0] == repo {
			d.Spec.Template.Spec.Containers[idx].Image = repo + ":" + newTag

			if retriesRemaining > 1 {
				d_new, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
				Logger.Sugar().Debugf("d_new.Spec.Template.Spec.Containers[0]: %v", d_new.Spec.Template.Spec.Containers[0])
				return nil
			} else {
				Logger.Warn("last retry ... (soft)forcing it")
				errr := errors.New("dummy")
				for errr != nil {
					_, errr = cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d, metav1.UpdateOptions{})
					if errr != nil {
						Logger.Sugar().Warn("faild to set newTag <%v> to <%v>, will retry", newTag, containerInfo)
						time.Sleep(30 * time.Second)
					}
				}
				return nil
			}
		} else {
			Logger.Sugar().Debugf("skip container: this == <%v>, looking for <%v>", imgNameTagArr[0], repo)
		}
	}
	return errors.New("did not find repo name: " + repo + ", failed to deploy newTag: " + newTag)
}
