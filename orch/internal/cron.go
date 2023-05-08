package internal

import (
	"fmt"
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
