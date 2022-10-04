package internal

import (
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
		t := time.Tick(c.Interval)
		for next := range t {
			Logger.Debug("Cron job: <" + c.Name + "," + c.Interval.String() + "> tick @ " + next.String())
			for name, job := range c.Jobs {
				Logger.Debug("running: " + name)
				job(c.Interval)
			}
		}
		time.Sleep(c.Interval)
	}()
	c.IsRunning = true
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
