package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Cronjob_dummy(interval string) {
	Logger.Debug("hello from Cronjob_dummy, interval=" + interval)
}

var pauseJob_idleCnt time.Duration

func Cronjob_pauseHC(interval time.Duration) {
	Logger.Debug("hello from Cronjob_pauseJob")
	//get ret_ccu

	retccu, err := getRetCcu()
	if err != nil {
		Logger.Error("retCcuReq err: " + err.Error())
		return
	}
	Logger.Sugar().Debugf("retCcu: %v", retccu)
	// resp, err := http.Client{Timeout:5*time.Millisecond, }

	if retccu != 0 {
		pauseJob_idleCnt = 0
	} else {
		pauseJob_idleCnt += interval
		Logger.Sugar().Debugf("updated pauseJob_idle: %v, time to pause: %v", pauseJob_idleCnt, (cfg.FreeTierIdleMax - pauseJob_idleCnt))
		if pauseJob_idleCnt >= cfg.FreeTierIdleMax {
			//pause it
			Logger.Info("Cronjob_pauseHC --- pausing -- " + cfg.PodNS)
			pauseReqBody, _ := json.Marshal(map[string]string{
				"hub_id": strings.TrimPrefix(cfg.PodNS, "hc-"),
			})
			pauseReq, err := http.NewRequest("PATCH", "https://"+cfg.turkeyorchHost+"/hc_instance?status=down", bytes.NewBuffer(pauseReqBody))
			if err != nil {
				Logger.Error("pauseReq err: " + err.Error())
				return
			}
			_, err = _httpClient.Do(pauseReq)
			if err != nil {
				Logger.Error("pauseReq err: " + err.Error())
				return
			}
		}
	}

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

	//get list of HC namespaces
	nsList, err := cfg.K8sClientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{LabelSelector: "hub_id"})
	if err != nil {
		Logger.Error(err.Error())
		return
	}
	//check them
	for _, ns := range nsList.Items {
		//get local endpoints from ingress
		Logger.Warn("comming soon -- ns: " + ns.Name)
	}

	//extra health checks
	for _, url := range cfg.ExtraHealthchecks {
		if url == "" {
			Logger.Warn("empty url")
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
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("bad resp: " + resp.Status)
	}
	return nil
}
