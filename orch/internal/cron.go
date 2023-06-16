package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Logger.Sugar().Infof("loaded job: %v", name)
	c.Jobs[name] = job
}

func (c *Cron) Start() *time.Ticker {
	if c.IsRunning {
		Logger.Warn("already running")
		return nil
	}
	if len(c.Jobs) == 0 {
		Logger.Warn("no jobs")
		return nil
	}

	if c.Interval <= 10*time.Second {
		Logger.Sugar().Warnf("c.Interval too small -- will use default: %v", defaultCronInterval)
		c.Interval = defaultCronInterval
	}
	Logger.Info("starting cron jobs, interval = " + c.Interval.String() + ", jobs: " + fmt.Sprint(len(c.Jobs)))
	ticker := time.NewTicker(c.Interval)
	go func() {
		// t := time.Tick(c.Interval)
		t := ticker.C
		for next := range t {
			Logger.Debug("Cron ... name: <" + c.Name + ", interval: " + c.Interval.String() + ", jobs: " + fmt.Sprint(len(c.Jobs)) + ", tick @ " + next.String())
			for name, job := range c.Jobs {
				Logger.Debug("running: " + name)
				go job(c.Interval)
			}
		}
		time.Sleep(c.Interval)
	}()
	c.IsRunning = true
	return ticker
}

// ############################################################################################
// ############################################# jobs #########################################
// ############################################################################################

func Cronjob_dummy(interval time.Duration) {
	Logger.Debug("hello from Cronjob_dummy")
}

// func Cronjob_TurkeyJobQueue(interval time.Duration) {
// 	Logger.Debug("hello from Cronjob_TurkeyJobQueue")

// 	msgBytes, err := Cfg.Gcps.PubSub_Pulling("turkey_job_queue_" + Cfg.ClusterName + "_sub")
// 	if err != nil {
// 		Logger.Sugar().Errorf("failed -- err: ", err)
// 	}
// 	Logger.Sugar().Debugf("msg received: %v", msgBytes)

// }

var HC_Count int32

func Cronjob_CountHC(interval time.Duration) {
	tStart := time.Now()
	nsList, err := Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{
		LabelSelector: "hub_id",
	})
	if err != nil {
		Logger.Error("Cronjob_CountHC failed: " + err.Error())
	}
	atomic.StoreInt32(&HC_Count, (int32)(len(nsList.Items)))
	Logger.Sugar().Debugf("Cronjob_CountHC took: %v", time.Since(tStart))

	//phone home
	Logger.Sugar().Debugf("[PeerReportWebhook] phone home: %v", Cfg.PeerReportWebhook)

	// token := PwdGen(64, time.Now().Unix(), "=")
	token := uuid.New().String()
	//add token to token book
	TokenBook.NewToken(token)

	jsonPayload, _ := json.Marshal(PeerReport{
		Domain: Cfg.Domain,

		Region:     Cfg.Region,
		HC_count:   int(HC_Count),
		T_unix_sec: time.Now().Unix(),
		Token:      token,
	})
	_, err = http.Post(Cfg.PeerReportWebhook, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		Logger.Error("[PeerReportWebhook] failed: " + err.Error())
		return
	}

}
