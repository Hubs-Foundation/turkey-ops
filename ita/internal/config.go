package internal

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var cfg *Config

// Config holds the runtime application config
type Config struct {
	PodNS  string
	Domain string `turkey domain`

	ListeningChannel  string
	SupportedChannels map[string]bool

	K8sCfg       *rest.Config
	K8sClientSet *kubernetes.Clientset

	TurkeyUpdater *TurkeyUpdater
}

func MakeCfg() {
	cfg = &Config{}
	cfg.SupportedChannels = map[string]bool{
		"dev":    true,
		"beta":   true,
		"stable": true,
	}

	cfg.Domain = os.Getenv("DOMAIN")
	cfg.ListeningChannel = os.Getenv("CHANNEL")
	_, ok := cfg.SupportedChannels[cfg.ListeningChannel]
	if !ok {
		Logger.Warn("bad env var CHANNEL: " + cfg.ListeningChannel + ", so we'll use stable")
		cfg.ListeningChannel = "stable"
	}

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

	//listening to stable channel by default
	cfg.TurkeyUpdater = NewTurkeyUpdater()
	_, err = cfg.TurkeyUpdater.Start()
	if err != nil {
		Logger.Error(err.Error())
	}

}
