package internal

import (
	"time"
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
