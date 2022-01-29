package internal

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var cfg *Config

// Config holds the runtime application config
type Config struct {
	PodNS string

	Env    string `long:"environment" env:"ENV" description:"env config"`
	Domain string `turkey domain`

	K8sCfg       *rest.Config
	K8sClientSet *kubernetes.Clientset

	TurkeyUpdater *TurkeyUpdater
}

func MakeCfg() {
	cfg = &Config{}
	cfg.Domain = os.Getenv("DOMAIN")

	var err error

	cfg.K8sCfg, err = rest.InClusterConfig()
	if err != nil {
		Logger.Error(err.Error())
	}

	cfg.PodNS = os.Getenv("POD_NS")
	if cfg.PodNS == "" {
		Logger.Error("POD_NS not set")
	}
	cfg.K8sClientSet, err = kubernetes.NewForConfig(cfg.K8sCfg)
	if err != nil {
		Logger.Error(err.Error())
	}

	cfg.TurkeyUpdater = NewTurkeyUpdater("dev")
	_, err = cfg.TurkeyUpdater.Start()
	if err != nil {
		Logger.Error(err.Error())
	}

}
