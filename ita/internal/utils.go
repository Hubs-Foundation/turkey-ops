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
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
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
	if cfg.PodDeploymentName == "" {
		return "", errors.New("cfg.PodDeploymentName is empty")
	}
	//do we have channel labled on deployment?
	d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), cfg.PodDeploymentName, metav1.GetOptions{})
	if err != nil {
		Logger.Sugar().Debugf("Deployment_getLabel failed: %v (cfg.PodDeploymentName: %v)", err, cfg.PodDeploymentName)
		return "", err
	}
	return d.Labels[key], nil
}

func Deployment_setLabel(key, val string) error {

	cfg.K8Man.WorkBegin(fmt.Sprintf("Deployment_setLabel: %v=%v", key, val))
	defer cfg.K8Man.WorkEnd("Deployment_setLabel")

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

func k8s_mountRetNfs(targetDeploymentName, volPathSubdir, mountPath string, readonly bool, propagation corev1.MountPropagationMode) error {

	cfg.K8Man.WorkBegin(fmt.Sprintf("k8s_mountRetNfs to %v, volPathSubdir: %v, mountPath: %v", targetDeploymentName, volPathSubdir, mountPath))
	defer cfg.K8Man.WorkEnd("k8s_mountRetNfs")

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
								MountPropagation: &propagation,
								ReadOnly:         readonly,
							},
						)
						// var_true := true
						// d_target.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
						// 	Privileged: &var_true,
						// }
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

	cfg.K8Man.WorkBegin("k8s_removeNfsMount for: " + targetDeploymentName)
	defer cfg.K8Man.WorkEnd("k8s_removeNfsMount")

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
	Logger.Debug("UnzipTar: " + src + ", destDir: " + destDir)

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
		if strings.HasPrefix(header.Name, `./`) {
			header.Name = header.Name[1:]
		}
		if !strings.HasPrefix(header.Name, `/`) {
			header.Name = `/` + header.Name
		}
		switch header.Typeflag {
		case tar.TypeDir:
			Logger.Debug("TypeDir: " + destDir + header.Name)
			if err := os.MkdirAll(destDir+header.Name, 0755); err != nil {
				return fmt.Errorf("Mkdir() failed: %s", err.Error())
			}
		case tar.TypeReg:
			Logger.Debug("TypeReg: " + destDir + header.Name)
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

	cfg.K8Man.WorkBegin("ingress_addItaApiRule")
	defer cfg.K8Man.WorkEnd("ingress_addItaApiRule")

	igs, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	ig, retRootRules, err := findIngressWithRetRootRules(&igs.Items)
	if err != nil {
		Logger.Error("findIngressWithRetRootPath failed: " + err.Error())
		return err
	}
	if ingressRuleAlreadyCreated_byBackendServiceName(ig, "ita") { // ingressRuleAlreadyCreated
		return nil
	}
	retRootRule := retRootRules[0]
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

func findIngressWithRetRootRules(igs *[]networkingv1.Ingress) (*networkingv1.Ingress, []networkingv1.IngressRule, error) {
	for _, ig := range *igs {
		retRootRule, err := findIngressRuleForRetRootPath(ig)
		if err == nil {
			return &ig, retRootRule, nil
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

func ingressRuleAlreadyCreated_byBackendHost(ig *networkingv1.Ingress, host string) (bool, *networkingv1.IngressRule) {
	for _, rule := range ig.Spec.Rules {
		if rule.Host == host {
			return true, &rule
		}
	}
	return false, nil
}

func findIngressRuleForRetRootPath(ig networkingv1.Ingress) ([]networkingv1.IngressRule, error) {
	r := []networkingv1.IngressRule{}
	for _, rule := range ig.Spec.Rules {
		if rule.HTTP.Paths[0].Path == "/" && rule.HTTP.Paths[0].Backend.Service.Name == "ret" {
			Logger.Sugar().Debugf("found: %v", rule)
			r = append(r, rule)
		}
	}
	if len(r) == 0 {
		return nil, errors.New("not found")
	}
	return r, nil
}

func pickLetsencryptAccountForHubId() string {
	accts, err := cfg.K8sClientSet.CoreV1().ConfigMaps("turkey-services").Get(context.Background(), "letsencrypt-accounts", metav1.GetOptions{})
	if err != nil {
		Logger.Error("failed to get letsencrypt-accounts CM, err: " + err.Error())
		return ""
	}
	if len(accts.Data) < 10 {
		Logger.Sugar().Warnf("will be making new letsencrypt acccount, %v is not enough", len(accts.Data))
		return ""
	}

	for _, v := range accts.Data { // random?
		return v
	}
	return ""

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
				Labels:    map[string]string{"app": "certbotbot-http"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "certbotbot",
						Image:           "mozillareality/certbotbot_http:stable-latest", //todo: <channel>-latest if channel's supported ?
						ImagePullPolicy: corev1.PullAlways,
						Env: []corev1.EnvVar{
							{Name: "DOMAIN", Value: customDomain},
							{Name: "NAMESPACE", Value: cfg.PodNS},
							{Name: "LETSENCRYPT_ACCOUNT", Value: letsencryptAcct},
							{Name: "CERT_NAME", Value: "cert-" + customDomain},
						},
					},
				},
				ServiceAccountName: "ita-sa",
				RestartPolicy:      "Never",
			},
		},
		metav1.CreateOptions{},
	)
	return err
}

func killPods(labelSelector string) error {
	Logger.Debug("killPods: " + labelSelector)
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

// func receiveFileFromReqForm(r *http.Request, expectedFileCount int) ([]string, error) {

// 	// 32 MB
// 	if err := r.ParseMultipartForm(32 << 20); err != nil {
// 		// http.Error(w, err.Error(), http.StatusBadRequest)
// 		return nil, err
// 	}

// 	Logger.Sugar().Debugf("r.MultipartForm.File: %v", r.MultipartForm.File)
// 	// get a reference to the fileHeaders
// 	files := r.MultipartForm.File["file"]

// 	if expectedFileCount != -1 && len(files) != expectedFileCount {
// 		return nil, errors.New("unexpected file count")
// 	}

// 	result := []string{}
// 	report := ""
// 	for _, fileHeader := range files {
// 		fileHeader = files[0]
// 		if fileHeader.Size > MAX_UPLOAD_SIZE {
// 			report += fmt.Sprintf("skipped(too big): %v(%v/%vMB)\n", fileHeader.Filename, fileHeader.Size, MAX_UPLOAD_SIZE/(1048576))
// 			result = append(result, "(skipped)"+fileHeader.Filename)
// 			continue
// 		}
// 		Logger.Sugar().Debugf("working on file: %v (%v)", fileHeader.Filename, fileHeader.Size)
// 		file, err := fileHeader.Open()
// 		if err != nil {
// 			report += fmt.Sprintf("failed to open %v, err: %v \n", fileHeader.Filename, err)
// 			result = append(result, "(failed to open)"+fileHeader.Filename)
// 			continue
// 		}
// 		defer file.Close()
// 		buff := make([]byte, 512)
// 		_, err = file.Read(buff)
// 		if err != nil {
// 			report += fmt.Sprintf("failed to read %v, err: %v \n", fileHeader.Filename, err)
// 			result = append(result, "(failed to read)"+fileHeader.Filename)
// 			continue
// 		}
// 		filetype := http.DetectContentType(buff)

// 		Logger.Debug("filetype: " + filetype)

// 		_, err = file.Seek(0, io.SeekStart)
// 		if err != nil {
// 			report += fmt.Sprintf("failed to seek %v, err: %v \n", fileHeader.Filename, err)
// 			result = append(result, "(failed to seek)"+fileHeader.Filename)
// 			continue
// 		}
// 		err = os.MkdirAll("/storage/ita_uploads", os.ModePerm)
// 		if err != nil {
// 			report += fmt.Sprintf("failed to makeDir %v, err: %v \n", fileHeader.Filename, err)
// 			result = append(result, "(failed to makeDir)"+fileHeader.Filename)
// 			continue
// 		}
// 		f, err := os.Create(fmt.Sprintf("/storage/ita_uploads/%s", fileHeader.Filename))
// 		if err != nil {
// 			report += fmt.Sprintf("failed to create %v, err: %v \n", fileHeader.Filename, err)
// 			result = append(result, "(failed to create)"+fileHeader.Filename)
// 			continue
// 		}
// 		defer f.Close()

// 		pg := &Progress{
// 			TotalSize: fileHeader.Size,
// 		}
// 		_, err = io.Copy(f, io.TeeReader(file, pg))
// 		if err != nil {
// 			report += fmt.Sprintf("failed to copy %v, err: %v \n", fileHeader.Filename, err)
// 			result = append(result, "(failed to copy)"+fileHeader.Filename)
// 			continue
// 		}
// 		report += fmt.Sprintf("saved: %v(%v, %vMB)\n", f.Name(), filetype, fileHeader.Size/(1024*1024))
// 		result = append(result, fileHeader.Filename)
// 	}

// 	Logger.Sugar().Debugf("report: %v", report)
// 	return result, nil
// }

func blockEgress(appName string) error {

	cfg.K8Man.WorkBegin("blockEgress for " + appName)
	defer cfg.K8Man.WorkEnd("blockEgress")

	npName := "egblock-" + appName

	_, err := cfg.K8sClientSet.NetworkingV1().NetworkPolicies(cfg.PodNS).Get(context.Background(), npName, metav1.GetOptions{})
	if err == nil {
		Logger.Info("already done: blockEgress for " + appName)
	}

	if err != nil && !k8errors.IsNotFound(err) {
		return err
	}

	_, err = cfg.K8sClientSet.NetworkingV1().NetworkPolicies(cfg.PodNS).Create(
		context.Background(),
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "egblock-" + appName,
				Namespace: cfg.PodNS,
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "app=" + appName}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			},
		},
		metav1.CreateOptions{},
	)

	return err
}

//curl -X POST -F file='@<path-to-file-ie-/tmp/file1>' ita:6000/upload
//curl -X POST -F file='@<path-to-file-ie-/tmp/file1>' -H 'addpath:/tmp' ita:6000/upload

func receiveFileFromReqBody(r *http.Request) ([]string, error) {
	Tstart := time.Now()
	Logger.Sugar().Debugf("handling an upload post")
	reader, err := r.MultipartReader()

	if err != nil {
		Logger.Sugar().Debugf("failed to get a multipart reader %v", err)
		return nil, err
	}

	baseDir := "/storage/ita_uploads"

	addPath := r.Header.Get("addpath")

	err = os.MkdirAll(baseDir+addPath, os.ModePerm)
	if err != nil {
		return nil, err
	}
	files := []string{}

	for {
		part, err := reader.NextPart()
		//no more files to process when io.EOF is found
		if err == io.EOF {
			Logger.Sugar().Debugf("EOF")
			break
		}

		//if part.FileName() is empty, skip this iteration.
		if part.FileName() == "" {
			Logger.Sugar().Debugf("empty filename, skip")
			continue
		}
		//create a timestamp
		//write the file to the fs
		dst, err := os.Create(baseDir + addPath + "/" + part.FileName())
		if err != nil {
			return nil, err
		}
		defer dst.Close()

		if err != nil {
			return nil, err
		}

		if _, err := io.Copy(dst, part); err != nil {
			return nil, err
		}
		files = append(files, part.FileName())
	}
	Logger.Sugar().Debugf("took: %v, file count: %v", time.Since(Tstart), len(files))

	if len(files) < 1 {
		return nil, errors.New("file not found")
	}

	return files, nil
}

func pointerOfInt32(i int) *int32 {
	int32i := int32(i)
	return &int32i
}

func HC_Pause() error {

	//back up current ingresses
	igs, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	igsbak, err := json.Marshal(*igs)
	if err != nil {
		Logger.Error("failed to marshal ingresses: " + err.Error())
	}

	igsbak_cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "igsbak"},
		BinaryData: map[string][]byte{"igsbak": igsbak},
	}
	_, err = cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Create(context.Background(), igsbak_cm, metav1.CreateOptions{})
	if err != nil {
		_, err = cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Update(context.Background(), igsbak_cm, metav1.UpdateOptions{})
		if err != nil {
			Logger.Error("failed to create/update ig_bak configmap:" + err.Error())
		}
	}

	//delete current ingresses
	for _, ig := range igs.Items {
		err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Delete(context.Background(), ig.Name, metav1.DeleteOptions{})
		if err != nil {
			Logger.Error("failed to delete ingresses" + err.Error())
		}
	}
	//create pausing ingress
	pathType_exact := networkingv1.PathTypeExact
	pathType_prefix := networkingv1.PathTypePrefix
	_, err = cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Create(context.Background(),
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "pausing",
				Annotations: map[string]string{"kubernetes.io/ingress.class": "haproxy"},
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						Host: cfg.SubDomain + "." + cfg.HubDomain,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathType_exact,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "ita",
												Port: networkingv1.ServiceBackendPort{
													Number: 6000,
												}}}},
									{
										Path:     "/z/resume",
										PathType: &pathType_prefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "ita",
												Port: networkingv1.ServiceBackendPort{
													Number: 6000,
												}}}},
								}}}}}},
		}, metav1.CreateOptions{})
	if err != nil {
		Logger.Error("failed to create pausing ingresses" + err.Error())
	}

	// scale down deployments, except ita
	ds, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error("failed to list deployments: " + err.Error())
		return err
	}
	for _, d := range ds.Items {
		if d.Name == "ita" {
			continue
		}
		d.Spec.Replicas = pointerOfInt32(0)
		_, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), &d, metav1.UpdateOptions{})
		if err != nil {
			Logger.Sugar().Errorf("failed to scale down %v: %v"+d.Name, err.Error())
			return err
		}
	}

	return nil
}

var resuming = int32(0)

func HC_Resume() error {
	if resuming != 0 {
		return errors.New("not yet")
	}
	atomic.StoreInt32(&resuming, 1)
	// scale back deployments
	ds, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error("failed to list deployments: " + err.Error())
		return err
	}
	for _, d := range ds.Items {
		if d.Name == "ita" {
			continue
		}
		d.Spec.Replicas = pointerOfInt32(1)
		_, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), &d, metav1.UpdateOptions{})
		if err != nil {
			Logger.Sugar().Errorf("failed to scale back %v: %v"+d.Name, err.Error())
			return err
		}
	}
	// delete pausing ingress
	err = cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Delete(context.Background(), "pausing", metav1.DeleteOptions{})
	if err != nil {
		Logger.Error("failed to delete pausing ingresses" + err.Error())
	}

	//wait for ret pod
	ret_readyReplicaCnt := 0
	ttl := 5 * time.Minute
	for ret_readyReplicaCnt < 1 && ttl > 0 {
		ret_d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), "reticulum", metav1.GetOptions{})
		if err != nil {
			Logger.Sugar().Errorf("failed to get reticulum deployment in ns %v", cfg.PodNS)
			time.Sleep(5 * time.Second)
			continue
		}
		ret_readyReplicaCnt = int(ret_d.Status.ReadyReplicas)
		Logger.Sugar().Debugf("waiting for ret, ttl: %v", ttl)
		time.Sleep(15 * time.Second)
		ttl -= 15 * time.Second
	}
	Logger.Debug("ret's ready")

	//restore ig_bak
	igsbak_cm, err := cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Get(context.Background(), "igsbak", metav1.GetOptions{})
	if err != nil {
		Logger.Error("failed to get ig_bak configmap:" + err.Error())
	}
	igsbak := igsbak_cm.BinaryData["igsbak"]
	var igs networkingv1.IngressList
	err = json.Unmarshal(igsbak, &igs)
	if err != nil {
		Logger.Sugar().Errorf("failed to unmarshal igsbak: %v", err)
	}
	for _, ig := range igs.Items {
		ig.ResourceVersion = ""
		_, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Create(context.Background(), &ig, metav1.CreateOptions{})
		if err != nil {
			Logger.Sugar().Errorf("failed to restore ig_bak: %v", err)
		}
	}

	go func() {
		time.Sleep(15 * time.Minute)
		atomic.StoreInt32(&resuming, 0)
	}()

	return nil
}
