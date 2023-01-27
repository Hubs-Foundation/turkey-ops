package internal

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"hash/fnv"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var Logger *zap.Logger
var Atom zap.AtomicLevel

func InitLogger() {
	Atom = zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "t"
	encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout("060102.03:04:05MST") //wanted to use time.Kitchen so much
	encoderCfg.CallerKey = "c"
	encoderCfg.FunctionKey = "f"
	encoderCfg.MessageKey = "m"
	// encoderCfg.FunctionKey = "f"
	Logger = zap.New(zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.Lock(os.Stdout), Atom), zap.AddCaller())

	defer Logger.Sync()

	if os.Getenv("LOG_LEVEL") == "warn" {
		Atom.SetLevel(zap.WarnLevel)
	} else if os.Getenv("LOG_LEVEL") == "debug" {
		Atom.SetLevel(zap.DebugLevel)
	} else {
		Atom.SetLevel(zap.InfoLevel)
	}
}

var listeningChannelLabelName = "CHANNEL"

func Get_listeningChannelLabel() (string, error) {
	//do we have channel labled on deployment?
	d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), cfg.PodDeploymentName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return d.Labels[listeningChannelLabelName], nil
}

func Set_listeningChannelLabel(channel string) error {
	//do we have channel labled on deployment?
	d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), cfg.PodDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	d.Labels["CHANNEL"] = channel
	_, err = cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

//internal request only, tls.insecureSkipVerify
var _httpClient = &http.Client{
	Timeout:   10 * time.Second,
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
}

func getRetCcu() (int, error) {
	retCcuReq, err := http.NewRequest("GET", "https://ret."+cfg.PodNS+":4000/api-internal/v1/presence", nil)
	retCcuReq.Header.Add("x-ret-dashboard-access-key", cfg.RetApiKey)
	if err != nil {
		return -1, err
	}

	resp, err := _httpClient.Do(retCcuReq)
	if err != nil {
		return -1, err

	}
	decoder := json.NewDecoder(resp.Body)

	var retCcuResp map[string]int
	err = decoder.Decode(&retCcuResp)
	if err != nil {
		Logger.Error("retCcuReq err: " + err.Error())
	}
	return retCcuResp["count"], nil
}

func waitRetCcu() error {
	timeout := 6 * time.Hour
	wait := 30 * time.Second
	timeWaited := 0 * time.Second
	for retCcu, _ := getRetCcu(); retCcu != 0; {
		time.Sleep(30 * time.Second)
		timeWaited += wait
		if timeWaited > timeout {
			return errors.New("timeout")
		}
	}
	return nil
}

func k8s_waitForDeployment(d *appsv1.Deployment, timeout time.Duration) (*appsv1.Deployment, error) {
	timeoutSec := timeout
	wait := 5 * time.Second
	for k8s_isDeploymentRunning(d) {
		Logger.Sugar().Debugf("waiting for %v -- currently: Replicas=%v, Available=%v, Ready=%v, Updated=%v",
			d.Name, d.Status.Replicas, d.Status.AvailableReplicas, d.Status.ReadyReplicas, d.Status.UpdatedReplicas)
		time.Sleep(wait)
		timeoutSec -= wait
		if timeoutSec < 0 {
			return d, errors.New("timeout while waiting for deployment <" + d.Name + ">")
		}
	}
	time.Sleep(wait) // time for k8s master services to sync, should be more than enough, or we'll get pending pods stuck forever
	return d, nil
}

func k8s_isDeploymentRunning(d *appsv1.Deployment) bool {
	d, _ = cfg.K8sClientSet.AppsV1().Deployments(d.Namespace).Get(context.Background(), d.Name, metav1.GetOptions{})
	if d.Status.Replicas != d.Status.AvailableReplicas ||
		d.Status.Replicas != d.Status.ReadyReplicas ||
		d.Status.Replicas != d.Status.UpdatedReplicas {
		return true
	}
	return false
}

func k8s_waitForPods(pods *corev1.PodList, timeout time.Duration) error {
	timeoutSec := timeout
	wait := 5 * time.Second
	for _, pod := range pods.Items {
		podStatusPhase := pod.Status.Phase
		for podStatusPhase == corev1.PodPending {
			Logger.Sugar().Debugf("waiting for pending pod %v / %v", pod.Namespace, pod.Name)
			time.Sleep(wait)
			timeoutSec -= wait
			pod, err := cfg.K8sClientSet.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			podStatusPhase = pod.Status.Phase
			if timeoutSec < 0*time.Second {
				return errors.New("timeout while waiting for pod: " + pod.Name + " in ns: " + pod.Namespace)
			}
		}
	}
	return nil
}

func k8s_mountRetNfs(targetDeploymentName string) error {

	d_target, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), targetDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	d_ret, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), "reticulum", metav1.GetOptions{})
	if err != nil {
		return err
	}

	targetHasVolume := false
	for _, v := range d_target.Spec.Template.Spec.Volumes {
		if v.Name == "nfs" {
			targetHasVolume = true
			break
		}
	}
	if !targetHasVolume {
		for _, v := range d_ret.Spec.Template.Spec.Volumes {
			if v.Name == "nfs" {
				d_target.Spec.Template.Spec.Volumes = append(
					d_target.Spec.Template.Spec.Volumes,
					v1.Volume{
						Name:         "nfs",
						VolumeSource: v1.VolumeSource{NFS: &v1.NFSVolumeSource{Server: v.NFS.Server, Path: v.NFS.Path}},
					})
			}
		}
	}

	targetHasMount := false
	for _, c := range d_target.Spec.Template.Spec.Containers {
		for _, vm := range c.VolumeMounts {
			if vm.Name == "nfs" {
				targetHasMount = true
				break
			}
		}
	}
	if !targetHasMount {
		for _, c := range d_ret.Spec.Template.Spec.Containers {
			if c.Name == "reticulum" {
				for _, vm := range c.VolumeMounts {
					if vm.Name == "nfs" {
						d_target.Spec.Template.Spec.Containers[0].VolumeMounts = append(
							d_target.Spec.Template.Spec.Containers[0].VolumeMounts,
							v1.VolumeMount{
								Name:             vm.Name,
								MountPath:        vm.MountPath,
								MountPropagation: vm.MountPropagation,
							},
						)
					}
				}
			}
		}
	}

	if !targetHasVolume || !targetHasMount {
		_, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d_target, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
