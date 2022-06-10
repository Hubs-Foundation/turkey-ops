package internal

import (
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var cfg *Config

// Config holds the runtime application config
type Config struct {
	PodNS             string
	PodDeploymentName string
	Domain            string //turkey domain
	Tier              string
	FreeTierIdleMax   time.Duration

	RetApiKey      string
	turkeyorchHost string

	SupportedChannels map[string]bool

	K8sCfg       *rest.Config
	K8sClientSet *kubernetes.Clientset

	TurkeyUpdater *TurkeyUpdater

	HostnameHash uint32
}

func GetCfg() *Config {
	return cfg
}

func MakeCfg() {
	cfg = &Config{}
	cfg.SupportedChannels = map[string]bool{
		"dev":    true,
		"beta":   true,
		"stable": true,
	}
	Hostname, err := os.Hostname()
	if err != nil {
		Logger.Warn("failed to get Hostname")
	} else {
		Logger.Debug("Hostname: " + Hostname)
	}
	cfg.HostnameHash = hash(Hostname)

	cfg.K8sCfg, err = rest.InClusterConfig()
	if err != nil {
		Logger.Error(err.Error())
	}
	cfg.K8sClientSet, err = kubernetes.NewForConfig(cfg.K8sCfg)
	if err != nil {
		Logger.Error(err.Error())
	}

	cfg.Domain = os.Getenv("DOMAIN")
	cfg.Tier = getEnv("TIER", "free")
	cfg.RetApiKey = getEnv("RET_API_KEY", "probably not this")
	cfg.turkeyorchHost = getEnv("TURKEYORCH_HOST", "turkeyorch.turkey-services:889")
	cfg.FreeTierIdleMax, err = time.ParseDuration(os.Getenv("FreeTierIdleMax"))
	if err != nil {
		cfg.FreeTierIdleMax = 30 * time.Minute
	}
	Logger.Sugar().Infof("cfg.turkeyorchHost: %v", cfg.turkeyorchHost)
	Logger.Sugar().Infof("cfg.FreeTierIdleMax: %v", cfg.FreeTierIdleMax)

	cfg.PodNS = os.Getenv("POD_NS")
	if cfg.PodNS == "" {
		Logger.Error("POD_NS not set")
	}
	val, retMode := os.LookupEnv("RET_MODE")
	if retMode {
		Logger.Info("RET_MODE: " + val)
		return
	}

	cfg.PodDeploymentName = getEnv("POD_DEPLOYMENT_NAME", "ita")

	listeningChannel, err := Get_listeningChannelLabel()
	if err != nil {
		Logger.Warn("Get_listeningChannelLabel failed: " + err.Error())
		listeningChannel = "unset"
		err := Set_listeningChannelLabel("unset")
		if err != nil {
			Logger.Error("Set_listeningChannelLabel failed: " + err.Error())
		}
	}

	cfg.TurkeyUpdater = NewTurkeyUpdater()
	err = cfg.TurkeyUpdater.Start(listeningChannel)
	if err != nil {
		Logger.Error(err.Error())
	}

}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
