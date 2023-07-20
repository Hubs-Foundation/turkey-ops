package internal

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Cronjob_dummy(interval string) {
	Logger.Debug("hello from Cronjob_dummy, interval=" + interval)
}

var pauseJob_idleCnt time.Duration

// var pausing bool

func Cronjob_pauseHC(interval time.Duration) {
	// if pausing {
	// 	return
	// }
	shouldPause := false

	// ~~~~~~~~~~~~ tmp ~~~~~~~~~~~~
	if tPaused_str, err := Deployment_getLabel("paused"); err == nil {
		Deployment_setLabel("paused", "")
		Logger.Debug("tPaused_str: " + tPaused_str)
		if tPaused_str != "" {
			if tPaused, err := time.Parse("060102", tPaused_str); err == nil {
				Logger.Sugar().Debugf("tPaused: %v", tPaused)

				rand.Seed(int64(cfg.HostnameHash))
				waitSec := rand.Intn(600)
				Logger.Sugar().Debugf("~~~tmp~~~pausing~~~start in %v secs", waitSec)
				time.Sleep(time.Duration(waitSec) * time.Second)

				Logger.Info("Cronjob_pauseHC --- pausing -- " + cfg.PodNS)

				err := orchCollect()
				if err != nil {
					Logger.Sugar().Errorf("failed: %v", err)
					return
				}

			}
		}
	}
	// ~~~~~~~~~~~~ tmp ~~~~~~~~~~~~

	//get ret_ccu
	retccu, err := getRetCcu()
	if err != nil {
		// Logger.Sugar().Debugf("retCcuReq err (%v), using retccu=1", err.Error())
		retccu = 1
	}
	Logger.Sugar().Debugf("retCcu: %v", retccu)
	if retccu != 0 {
		pauseJob_idleCnt = 0
	} else {
		pauseJob_idleCnt += interval
		Logger.Sugar().Debugf("idle: %v, time to pause: %v", pauseJob_idleCnt, (cfg.FreeTierIdleMax - pauseJob_idleCnt))

		shouldPause = pauseJob_idleCnt >= cfg.FreeTierIdleMax
	}

	if shouldPause {
		Logger.Info("Cronjob_pauseHC --- pausing -- " + cfg.PodNS)

		err := orchCollect()
		if err != nil {
			Logger.Sugar().Errorf("failed: %v", err)
			return
		}
		// pausing = true
	}

}

func orchCollect() error {
	hub_id := strings.Split(cfg.PodNS, "-")[1]
	// data := fmt.Sprintf(`{"hub_id": "%v", "subdomain":"%v","tier":"%v","useremail":"%v","guardiankey":"%v","phxkey":"%v"}`,
	// 	hub_id, cfg.SubDomain, cfg.Tier, cfg.RootUserEmail, cfg.Ret_guardiankey, cfg.Ret_phxkey)

	data, _ := json.Marshal(map[string]string{
		"hub_id":      hub_id,
		"subdomain":   cfg.SubDomain,
		"tier":        cfg.Tier,
		"useremail":   cfg.RootUserEmail,
		"guardiankey": cfg.Ret_guardiankey,
		"phxkey":      cfg.Ret_phxkey,
	})

	req, err := http.NewRequest("PATCH", "http://"+cfg.turkeyorchHost+"/hc_instance?status=collect", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	var httpClient = &http.Client{Timeout: 300 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp == nil {
		return errors.New("something went wrong -- resp==nil")
	}
	if resp.StatusCode < 299 {
		return nil
	}
	body, _ := ioutil.ReadAll(resp.Body)

	return fmt.Errorf("bad resp, code: %v, body: %v", resp.StatusCode, string(body))
}

func Cronjob_publishTurkeyBuildReport(interval time.Duration) {
	bucket := "turkeycfg"
	for _, channel := range cfg.SupportedChannels {
		filename := "build-report-" + channel
		//read
		br, err := GCS_ReadFile(bucket, filename)
		if err != nil {
			Logger.Error(err.Error())
		}
		//make brMap
		brMap := make(map[string]string)
		err = json.Unmarshal(br, &brMap)
		if err != nil {
			Logger.Error(err.Error())
		}
		Logger.Sugar().Debugf("publishing: channel: %v brMap: %v", channel, brMap)
		//publish
		err = publishToConfigmap_label(channel, brMap)
		if err != nil {
			Logger.Error("failed to publishToConfigmap_label: " + err.Error())
		}
	}
}

func GCS_ReadFile(bucketName, filename string) ([]byte, error) {
	Logger.Debug("reading bucket: " + bucketName + ", key: " + filename)
	client, err := storage.NewClient(context.Background())
	if err != nil {
		return nil, err
	}
	obj := client.Bucket(bucketName).Object(filename)
	r, err := obj.NewReader(context.Background())
	if err != nil {
		return nil, err
	}
	defer r.Close()
	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func Cronjob_cleanupFailedPods(interval time.Duration) {
	nsList, err := cfg.K8sClientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error(err.Error())
		return
	}
	for _, ns := range nsList.Items {
		failedPods, err := cfg.K8sClientSet.CoreV1().Pods(ns.Name).List(context.Background(), metav1.ListOptions{FieldSelector: "status.phase=Failed"})
		if err != nil {
			Logger.Error(err.Error())
			return
		}
		failedPodsCnt := len(failedPods.Items)
		if failedPodsCnt > 0 {
			Logger.Sugar().Infof("deleting %v failed pods in ns: %v", failedPodsCnt, ns.Name)
		}
		for _, failedPod := range failedPods.Items {
			err := cfg.K8sClientSet.CoreV1().Pods(ns.Name).Delete(context.Background(), failedPod.Name, metav1.DeleteOptions{})
			if err != nil {
				Logger.Error(err.Error())
			}
		}
	}
}

func publishToConfigmap_label(channel string, repo_tag_map map[string]string) error {
	cfg.K8Man.WorkBegin("publishToConfigmap_label, channel == " + channel)
	defer cfg.K8Man.WorkEnd("publishToConfigmap_label")

	cfgmap, err := cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Get(context.Background(), "hubsbuilds-"+channel, metav1.GetOptions{})
	if err != nil {
		Logger.Error(err.Error())
	}
	for k, v := range repo_tag_map {
		cfgmap.Labels[k] = v
	}
	_, err = cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Update(context.Background(), cfgmap, metav1.UpdateOptions{})
	return err
}

func Cronjob_HcHealthchecks(interval time.Duration) {

	// //get list of HC namespaces
	// nsList, err := cfg.K8sClientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{LabelSelector: "hub_id"})
	// if err != nil {
	// 	Logger.Error(err.Error())
	// 	return
	// }
	// //check them
	// for _, ns := range nsList.Items {
	// 	//get local endpoints from ingress
	// 	// Logger.Warn("comming soon -- ns: " + ns.Name)
	// 	_ = ns
	// }

	// ns, err := cfg.K8sClientSet.CoreV1().Namespaces().Get(context.Background(), cfg.PodNS, metav1.GetOptions{})
	// if err != nil {
	// 	Logger.Error("failed to get ns: " + err.Error())
	// }

	hcHost, err := get_hc_host()
	if err != nil {
		Logger.Error(err.Error())
		return
	}
	err = healthcheckUrl("https://" + hcHost)
	if err != nil {
		Logger.Error("unhealthy: <" + hcHost + "> " + err.Error())
	}

	//extra health checks
	for _, url := range cfg.ExtraHealthchecks {
		if url == "" {
			// Logger.Warn("empty url")
			continue
		}
		Logger.Debug("url: " + url)
		err := healthcheckUrl(url)
		if err != nil {
			Logger.Error("unhealthy: <" + url + "> " + err.Error())
		}
	}
}

func healthcheckUrl(url string) error {

	// resp, err := http.Get(url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Cache-Control", "no-cache")
	resp, err := _httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("bad resp: " + resp.Status)
	}

	Logger.Debug("good url: " + url)

	return nil
}

var StreamNodes map[string]string

var StreamNodeIpList string
var mu_streamNodes sync.Mutex

func Cronjob_SurveyStreamNodes(interval time.Duration) {

	r := make(map[string]string)

	nodeIps := make(map[string]string)
	cfg.K8sClientSet.NodeV1().RESTClient().Get()

	nodes, err := cfg.K8sClientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error(err.Error())
	}
	for _, node := range nodes.Items {
		// nodeLabels := node.GetObjectMeta().GetLabels()
		// Logger.Sugar().Debugf("nodeLabels: %v", nodeLabels)
		// nodePool := nodeLabels["turkey"]
		// if nodePool == "stream" || nodePool=="service" {
		nodePubIp := "?"
		for _, addr := range node.Status.Addresses {
			if addr.Type == "ExternalIP" {
				nodePubIp = addr.Address
			}
		}
		nodeIps[node.Name] = nodePubIp
		// }
	}
	coturnPods, _ := cfg.K8sClientSet.CoreV1().Pods("turkey-stream").List(context.Background(), metav1.ListOptions{LabelSelector: "app=coturn"})
	for _, pod := range coturnPods.Items {
		r[nodeIps[pod.Spec.NodeName]] = "coturn"
	}
	dialogPods, _ := cfg.K8sClientSet.CoreV1().Pods("turkey-stream").List(context.Background(), metav1.ListOptions{LabelSelector: "app=dialog"})
	for _, pod := range dialogPods.Items {
		r[nodeIps[pod.Spec.NodeName]] = "dialog"
	}

	//it shouldn't change anyway but --- todo -- get them from GCP
	r["35.225.11.240"] = "hmc-gke-lb"
	r["34.111.227.97"] = "hmc-gke-ig"

	ipList := ""
	for ip, _ := range r {
		ipList += ip + "\n"
	}
	r["_as-of"] = time.Now().Format(time.UnixDate)

	mu_streamNodes.Lock()
	StreamNodes = r
	StreamNodeIpList = ipList
	mu_streamNodes.Unlock()
}

func get_hc_host() (string, error) {
	resp, err := http.Get("http://ret." + cfg.PodNS + ":4001/api/v1/meta")
	if err != nil {
		return "", err
	}
	rBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	rMap := make(map[string]string)
	err = json.Unmarshal(rBody, &rMap)
	if err != nil {
		return "", err
	}
	Logger.Debug("phx_host: " + rMap["phx_host"])
	return rMap["phx_host"], nil

}

func Cronjob_customDomainCert(interval time.Duration) {

	if cfg.CustomDomainCertExp.IsZero() {
		secret, err := cfg.K8sClientSet.CoreV1().Secrets(cfg.PodNS).Get(context.Background(), "cert-"+cfg.CustomDomain, metav1.GetOptions{})
		if err != nil {
			Logger.Sugar().Errorf("failed to get secret: cert-%v. features: %+v", cfg.CustomDomain, cfg.Features.Get())
			return
		}
		certData, ok := secret.Data["tls.crt"]
		if !ok {
			Logger.Sugar().Errorf("failed to find cert in  secret.Data[\"tls.crt\"] (%v)", string(certData))
			return
		}

		// Parse the certificate
		block, _ := pem.Decode(certData)
		if block == nil {
			panic("Failed to decode certificate data")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			panic(err)
		}

		// Get the expiration date of the certificate
		cfg.CustomDomainCertExp = cert.NotAfter
		Logger.Sugar().Infof("cfg.CustomDomainCertExp refreshed to: ", cfg.CustomDomainCertExp)
	}

	certTtl := time.Until(cfg.CustomDomainCertExp)
	Logger.Sugar().Infof("customDomain cert (cert-%v) Ttl: %v", cfg.CustomDomain, certTtl)

	if certTtl < 30*24*time.Hour {
		Logger.Sugar().Infof("renewing CustomDomain cert: cert-%v", cfg.CustomDomain)
		err := CustomDomain_UpdateCert()
		if err != nil {
			Logger.Sugar().Errorf("failed @ CustomDomain_UpdateCert: %v", err)
		}
		cfg.CustomDomainCertExp = time.Time{}
	}

}
