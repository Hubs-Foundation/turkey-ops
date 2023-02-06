package internal

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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

func Deployment_getLabel(key string) (string, error) {
	//do we have channel labled on deployment?
	d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), cfg.PodDeploymentName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return d.Labels[key], nil
}

func Deployment_setLabel(key, val string) error {
	//do we have channel labled on deployment?
	d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), cfg.PodDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	d.Labels[key] = val
	_, err = cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// func NS_setLabel(key, val string) error {
// 	ns, err := cfg.K8sClientSet.CoreV1().Namespaces().Get(context.Background(), cfg.PodNS, metav1.GetOptions{})
// 	if err != nil {
// 		return err
// 	}
// 	ns.Labels[key] = val

// 	_, err = cfg.K8sClientSet.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})

// 	return err
// }

func NS_getLabel(key string) (string, error) {
	ns, err := cfg.K8sClientSet.CoreV1().Namespaces().Get(context.Background(), cfg.PodNS, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return ns.Labels[key], nil
}

func Get_fromNsAnnotations(key string) (string, error) {
	ns, err := cfg.K8sClientSet.CoreV1().Namespaces().Get(context.Background(), cfg.PodNS, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return ns.Annotations[key], nil
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

	wait := 5 * time.Second
	for k8s_isDeploymentRunning(d) {
		Logger.Sugar().Debugf("waiting for %v -- currently: Replicas=%v, Available=%v, Ready=%v, Updated=%v",
			d.Name, d.Status.Replicas, d.Status.AvailableReplicas, d.Status.ReadyReplicas, d.Status.UpdatedReplicas)
		time.Sleep(wait)
		timeout -= wait
		if timeout < 0 {
			return d, errors.New("timeout while waiting for deployment <" + d.Name + ">")
		}
	}
	time.Sleep(wait) // time for k8s master services to sync, should be more than enough, or we'll get pending pods stuck forever
	//return refreshed deployment
	d, _ = cfg.K8sClientSet.AppsV1().Deployments(d.Namespace).Get(context.Background(), d.Name, metav1.GetOptions{})
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

	wait := 5 * time.Second
	for _, pod := range pods.Items {
		podStatusPhase := pod.Status.Phase
		for podStatusPhase == corev1.PodPending {
			Logger.Sugar().Debugf("waiting for pending pod %v / %v", pod.Namespace, pod.Name)
			time.Sleep(wait)
			timeout -= wait
			pod, err := cfg.K8sClientSet.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			podStatusPhase = pod.Status.Phase
			if timeout < 0*time.Second {
				return errors.New("timeout while waiting for pod: " + pod.Name + " in ns: " + pod.Namespace)
			}
		}
	}
	return nil
}

func k8s_mountRetNfs(targetDeploymentName, volPathSubdir, mountPath string) error {
	Logger.Debug("mounting Ret nfs for: " + targetDeploymentName)

	d_target, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), targetDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	d_target, err = k8s_waitForDeployment(d_target, 2*time.Minute)
	if err != nil {
		return err
	}

	if len(d_target.Spec.Template.Spec.Containers) > 1 {
		return errors.New("this won't work because d_target.Spec.Template.Spec.Containers != 1")
	}

	d_ret, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), "reticulum", metav1.GetOptions{})
	if err != nil {
		return err
	}

	targetHasVolume := false
	for _, v := range d_target.Spec.Template.Spec.Volumes {
		if v.Name == "nfs" {
			targetHasVolume = true
			Logger.Sugar().Debugf("nfs volume already exist for Deployment: %v", targetDeploymentName)
			break
		}
	}
	if !targetHasVolume {
		for _, v := range d_ret.Spec.Template.Spec.Volumes {
			if v.Name == "nfs" {
				d_target.Spec.Template.Spec.Volumes = append(
					d_target.Spec.Template.Spec.Volumes,
					corev1.Volume{
						Name:         "nfs",
						VolumeSource: corev1.VolumeSource{NFS: &corev1.NFSVolumeSource{Server: v.NFS.Server, Path: v.NFS.Path + volPathSubdir}},
					})
			}
		}
	}

	targetHasMount := false
	for _, c := range d_target.Spec.Template.Spec.Containers {
		for _, vm := range c.VolumeMounts {
			if vm.Name == "nfs" {
				targetHasMount = true
				Logger.Sugar().Debugf("nfs volumeMount already exist for Deployment: %v, Container: %v",
					targetDeploymentName, c.Name)
				break
			}
		}
	}
	if !targetHasMount {
		for _, c := range d_ret.Spec.Template.Spec.Containers {
			if c.Name == "reticulum" {
				for _, vm := range c.VolumeMounts {
					if vm.Name == "nfs" {
						if mountPath == "" {
							mountPath = vm.MountPath
						}
						d_target.Spec.Template.Spec.Containers[0].VolumeMounts = append(
							d_target.Spec.Template.Spec.Containers[0].VolumeMounts,
							corev1.VolumeMount{
								Name:             vm.Name,
								MountPath:        mountPath,
								MountPropagation: vm.MountPropagation,
							},
						)
						var_true := true
						d_target.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
							Privileged: &var_true,
						}
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

func k8s_removeNfsMount(targetDeploymentName string) error {

	d_target, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), targetDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	volumes := []corev1.Volume{}
	for _, v := range d_target.Spec.Template.Spec.Volumes {
		if v.Name != "nfs" {
			volumes = append(volumes, v)
		}
	}
	d_target.Spec.Template.Spec.Volumes = volumes

	for idx, c := range d_target.Spec.Template.Spec.Containers {
		volumesMounts := []corev1.VolumeMount{}
		for _, vm := range c.VolumeMounts {
			if vm.Name != "nfs" {
				volumesMounts = append(volumesMounts, vm)
			}
		}
		d_target.Spec.Template.Spec.Containers[idx].VolumeMounts = volumesMounts
	}

	_, err = cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d_target, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("%v --- %v", err, d_target))
	}
	return nil
}

func k8s_KillPodsByLabel(label string) error {
	pods, err := cfg.K8sClientSet.CoreV1().Pods(cfg.PodNS).List(context.Background(), metav1.ListOptions{
		LabelSelector: label, // ie: app=hubs
	})
	if err != nil {
		return err
	}
	for _, p := range pods.Items {
		Logger.Info("deleting pod: " + p.Name)
		err := cfg.K8sClientSet.CoreV1().Pods(cfg.PodNS).Delete(context.Background(), p.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

// func ExtractTarGz(gzipStream io.Reader) error {
func UnzipTar(src, destDir string) error {

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open src file (%v), err: %v", src, err)
	}

	os.MkdirAll(destDir, 0755)

	uncompressedStream, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("NewReader failed, err: %v (is this a plain / non-gzipped tar ? try tar -czvf)", err)
	}
	tarReader := tar.NewReader(uncompressedStream)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Next() failed: %s", err.Error())
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destDir+header.Name, 0755); err != nil {
				return fmt.Errorf("Mkdir() failed: %s", err.Error())
			}
		case tar.TypeReg:
			outFile, err := os.Create(destDir + header.Name)
			if err != nil {
				return fmt.Errorf("Create() failed: %s", err.Error())
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return fmt.Errorf("Copy() failed: %s", err.Error())
			}
			outFile.Close()
		default:
			return fmt.Errorf("abort -- uknown type: %v in %v", header.Typeflag, header.Name)
		}
	}
	return nil
}

func UnzipZip(src, destDir string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(destDir, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(destDir, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func ingress_addItaApiRule() error {
	igs, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	ig, retRootRule, err := findIngressWithRetRootRule(&igs.Items)
	if err != nil {
		Logger.Error("findIngressWithRetRootPath failed: " + err.Error())
		return err
	}
	if ingressRuleAlreadyCreated_byBackendServiceName(ig, "ita") { // ingressRuleAlreadyCreated
		return nil
	}

	port := int32(6000)
	if _, ok := ig.Annotations["haproxy.org/server-ssl"]; ok {
		port = 6001
	}
	itaRule := retRootRule.DeepCopy()
	itaRule.HTTP.Paths[0].Path = "/api/ita"
	itaRule.HTTP.Paths[0].Backend.Service.Name = "ita"
	itaRule.HTTP.Paths[0].Backend.Service.Port.Number = port
	ig.Spec.Rules = append(ig.Spec.Rules, *itaRule)
	newIg, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Update(context.Background(), ig, metav1.UpdateOptions{})
	if err != nil {
		Logger.Sugar().Errorf("failed to update ingress with itaRule: %v", err)
		return err
	}
	Logger.Sugar().Debugf("updated ingress: %v", newIg)
	return nil
}

func ingress_addCustomDomainRule(customDomain string) error {
	igs, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	ig, retRootRule, err := findIngressWithRetRootRule(&igs.Items)
	if err != nil {
		Logger.Error("findIngressWithRetRootPath failed: " + err.Error())
		return err
	}
	if ingressRuleAlreadyCreated_byBackendHost(ig, customDomain) { // ingressRuleAlreadyCreated
		return nil
	}
	customDomainRule := retRootRule.DeepCopy()
	customDomainRule.Host = customDomain
	ig.Spec.Rules = append(ig.Spec.Rules, *customDomainRule)
	newIg, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Update(context.Background(), ig, metav1.UpdateOptions{})
	if err != nil {
		Logger.Sugar().Errorf("failed to update ingress with customDomainRule: %v", err)
		return err
	}
	Logger.Sugar().Debugf("updated ingress: %v", newIg)
	return nil
}
func findIngressWithRetRootRule(igs *[]networkingv1.Ingress) (*networkingv1.Ingress, *networkingv1.IngressRule, error) {
	for _, ig := range *igs {
		retRootRule, err := findIngressRuleForRetRootPath(ig)
		if err == nil {
			return &ig, &retRootRule, nil
		}
	}

	return nil, nil, errors.New("findIngressWithRetRootPath: not found")

}

func ingressRuleAlreadyCreated_byBackendServiceName(ig *networkingv1.Ingress, backendServiceName string) bool {
	for _, rule := range ig.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service.Name == backendServiceName {
				Logger.Sugar().Debugf("ingressRuleAlreadyCreated: %v", rule)
				return true
			}
		}
	}
	return false
}

func ingressRuleAlreadyCreated_byBackendHost(ig *networkingv1.Ingress, host string) bool {
	for _, rule := range ig.Spec.Rules {
		if rule.Host == host {
			return true
		}
	}
	return false
}

func findIngressRuleForRetRootPath(ig networkingv1.Ingress) (networkingv1.IngressRule, error) {
	for _, rule := range ig.Spec.Rules {
		if rule.HTTP.Paths[0].Path == "/" && rule.HTTP.Paths[0].Backend.Service.Name == "ret" {
			return rule, nil
		}
	}
	return networkingv1.IngressRule{}, errors.New("findIngressRuleForRetRootPath: not found")

}

func pickLetsencryptAccountForHubId() string {
	accts, err := cfg.K8sClientSet.CoreV1().ConfigMaps("turkey-services").Get(context.Background(), "letsencrypt-accounts", metav1.GetOptions{})
	if err != nil {
		return ""
	}

	for _, v := range accts.Data {
		return v

	}
	return ""
}

func ret_AddSecondaryUrl(url string) error {

	retCm, err := cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Get(context.Background(), "ret-config", metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Logger.Debug(`cm.Data["config.toml.template"]~~~~~~` + retCm.Data["config.toml.template"])

	retCm.Data["config.toml.template"] = strings.Replace(
		retCm.Data["config.toml.template"],
		`[ret."Elixir.RetWeb.Endpoint".secondary_url]

[`,
		`[ret."Elixir.RetWeb.Endpoint".secondary_url]
host = "`+url+`"
[`, 1)
	_, err = cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Update(context.Background(), retCm, metav1.UpdateOptions{})

	return err
}

func runCertbotbotpod(letsencryptAcct, customDomain string) error {

	if customDomain == "" {
		customDomain, _ = Deployment_getLabel("custom_domain")
	}

	_, err := cfg.K8sClientSet.CoreV1().Pods(cfg.PodNS).Create(
		context.Background(),
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("certbotbot-%v", time.Now().Unix()),
				Namespace: cfg.PodNS,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "certbotbot",
						Image: "mozillareality/certbotbot_http:17",
						Env: []corev1.EnvVar{
							{Name: "DOMAIN", Value: customDomain},
							{Name: "NAMESPACE", Value: cfg.PodNS},
							{Name: "LETSENCRYPT_ACCOUNT", Value: letsencryptAcct},
						},
					},
				},
				ServiceAccountName: "ita-sa",
			},
		},
		metav1.CreateOptions{},
	)

	return err

}

func killPods(labelSelector string) error {
	pods, err := cfg.K8sClientSet.CoreV1().Pods(cfg.PodNS).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		err := cfg.K8sClientSet.CoreV1().Pods(cfg.PodNS).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed while deleting <%v> %v", pod.Name, err)
		}
	}

	return nil
}
