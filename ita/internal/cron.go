package internal

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type Cron struct {
	Name      string
	Interval  time.Duration
	IsRunning bool
	Jobs      map[string]func(interval time.Duration)
}

var defaultCronInterval = 10 * time.Minute

func NewCron(name string, interval time.Duration) *Cron {
	return &Cron{
		Name:      name,
		Interval:  interval,
		IsRunning: false,
		Jobs:      make(map[string]func(interval time.Duration)),
	}
}

func (c *Cron) Load(name string, job func(interval time.Duration)) {
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
	if c.Interval <= time.Second {
		Logger.Sugar().Warnf("c.Interval too small -- will use default: %v", defaultCronInterval)
		c.Interval = defaultCronInterval
	}
	Logger.Info("starting cron jobs, interval = " + c.Interval.String())
	go func() {
		time.Sleep(c.Interval)
		t := time.Tick(c.Interval)
		for next := range t {
			Logger.Debug("Cron job: <" + c.Name + "," + c.Interval.String() + "> tick @ " + next.String())
			for name, job := range c.Jobs {
				Logger.Debug("running: " + name)
				job(c.Interval)
			}
		}
	}()
	c.IsRunning = true
}

func Cronjob_dummy(interval string) {
	Logger.Debug("hello from Cronjob_dummy, interval=" + interval)
}

var pauseJob_idleCnt time.Duration

func Cronjob_pauseJob(interval time.Duration) {
	Logger.Debug("hello from Cronjob_pauseJob")
	//get ret_ccu
	retCcuReq, err := http.NewRequest("GET", "https://ret."+cfg.PodNS+":4000/api-internal/v1/presence", nil)
	retCcuReq.Header.Add("x-ret-dashboard-access-key", cfg.RetApiKey)
	if err != nil {
		Logger.Error("retCcuReq err: " + err.Error())
		return
	}
	httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	} // todo: make one in utils? and reuse that
	resp, err := httpClient.Do(retCcuReq)
	if err != nil {
		Logger.Error("retCcuReq err: " + err.Error())
		return
	}
	decoder := json.NewDecoder(resp.Body)

	var retCcuResp map[string]int
	err = decoder.Decode(&retCcuResp)
	if err != nil {
		Logger.Error("retCcuReq err: " + err.Error())
	}
	retccu := retCcuResp["count"]
	Logger.Sugar().Debugf("retCcu: %v", retccu)
	// resp, err := http.Client{Timeout:5*time.Millisecond, }

	if retccu != 0 {
		pauseJob_idleCnt = 0
	} else {
		pauseJob_idleCnt += interval
		Logger.Sugar().Debugf("updated pauseJob_idle: %v, time to pause: %v", pauseJob_idleCnt, (cfg.FreeTierIdleMax - pauseJob_idleCnt))
		if pauseJob_idleCnt >= cfg.FreeTierIdleMax {
			//pause it
			pauseReqBody, _ := json.Marshal(map[string]string{
				"hub_id": strings.TrimPrefix(cfg.PodNS, "hc-"),
			})
			pauseReq, err := http.NewRequest("PATCH", "https://"+cfg.turkeyorchHost+"/hc_instance?status=down", bytes.NewBuffer(pauseReqBody))
			if err != nil {
				Logger.Error("pauseReq err: " + err.Error())
				return
			}
			_, err = httpClient.Do(pauseReq)
			if err != nil {
				Logger.Error("pauseReq err: " + err.Error())
				return
			}
		}
	}

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
