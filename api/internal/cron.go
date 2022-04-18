package internal

import (
	"context"
	"encoding/json"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Cron struct {
	Name      string
	Interval  string
	IsRunning bool
	Jobs      map[string]func()
}

var defaultCronInterval = "10m"

func NewCron(name, interval string) *Cron {
	return &Cron{
		Name:      name,
		Interval:  interval,
		IsRunning: false,
		Jobs:      make(map[string]func()),
	}
}

func (c *Cron) Load(name string, job func()) {
	c.Jobs[name] = job
}

func (c *Cron) Start() {
	if c.IsRunning {
		Logger.Warn("already running")
		return
	}
	if len(c.Jobs) == 0 {
		Logger.Warn("no jobs")
		return
	}
	// cron, non-stop, no way to stop it really ... add wrapper in future in case we need to stop it
	if c.Interval == "" {
		Logger.Warn("empty envVar CRON_INTERVAL, falling back to default: " + defaultCronInterval)
		c.Interval = defaultCronInterval
	}
	interval, err := time.ParseDuration(c.Interval)
	if err != nil {
		Logger.Warn("bad CRON_INTERVAL: " + c.Interval + " -- falling back to default: " + defaultCronInterval)
		c.Interval = defaultCronInterval
		interval, _ = time.ParseDuration(defaultCronInterval)
	}
	Logger.Info("starting cron jobs for -- name: " + c.Name + ", interval:" + c.Interval)
	go func() {
		time.Sleep(interval)
		t := time.Tick(interval)
		for next := range t {
			Logger.Debug("Cron job: <" + c.Name + "," + c.Interval + "> tick @ " + next.String())
			for name, job := range c.Jobs {
				Logger.Debug("running: " + name)
				job()
			}
		}
	}()
	c.IsRunning = true
}

// ############################################################################################
// ############################################# jobs #########################################
// ############################################################################################

func Cronjob_dummy() {
	Logger.Debug("hello from Cronjob_dummy")
}

func Cronjob_publishTurkeyBuildReport() {
	channel := Cfg.Channel
	bucket := "turkeycfg"

	filename := "build-report-" + channel
	//read
	br, err := Cfg.Gcps.GCS_ReadFile(bucket, filename)
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

	// //indexing (regroup brMap) for channels
	// brMap_chan := make(map[string]map[string]string)
	// for k, v := range brMap {
	// 	for _, channel := range []string{"dev, beta, stable"} {
	// 		if strings.HasPrefix(v, channel+"-") {
	// 			brMap_chan[channel][k] = v
	// 		} else {
	// 			Logger.Error("unexpected tag found in brMap[" + k + "]: " + v)
	// 		}
	// 	}
	// }
	// //publish them
	// for k, v := range brMap_chan {
	// 	err := publishToConfigmap_label(k, v)
	// 	if err != nil {
	// 		Logger.Error("failed to publishToConfigmap_label: " + err.Error())
	// 	}
	// }

}

func publishToConfigmap_label(channel string, repo_tag_map map[string]string) error {
	cfgmap, err := Cfg.K8ss_local.ClientSet.CoreV1().ConfigMaps(Cfg.PodNS).Get(context.Background(), "hubsbuilds-"+channel, metav1.GetOptions{})
	if err != nil {
		Logger.Error(err.Error())
	}
	for k, v := range repo_tag_map {
		cfgmap.Labels[k] = v
	}
	_, err = Cfg.K8ss_local.ClientSet.CoreV1().ConfigMaps(Cfg.PodNS).Update(context.Background(), cfgmap, metav1.UpdateOptions{})
	return err
}

// func Cronjob_updateDeployment(deploymentName string) {

// 	currentDeployment, err :=
// 		cfg.K8sClientSet.AppsV1().
// 			Deployments(cfg.K8sNamespace).Get(context.Background(), deploymentName, v1.GetOptions{})
// 	if err != nil {
// 		Logger.Error("failed to get deployment for <" + deploymentName + "> because: " + err.Error())
// 		return
// 	}
// 	currentImage := currentDeployment.Spec.Template.Spec.Containers[0].Image
// 	fmt.Println("currentImage: " + currentImage)

// }
