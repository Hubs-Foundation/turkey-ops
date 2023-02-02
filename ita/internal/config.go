package internal

import (
	"os"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/strings/slices"
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

	SupportedChannels []string
	Channel           string

	K8sCfg       *rest.Config
	K8sClientSet *kubernetes.Clientset

	TurkeyUpdater *TurkeyUpdater

	HostnameHash uint32

	ExtraHealthchecks []string

	Features      itaFeatures
	RootUserEmail string
}

func GetCfg() *Config {
	return cfg
}

func MakeCfg() {
	cfg = &Config{}

	// tmp hack
	val, retMode := os.LookupEnv("RET_MODE")
	if retMode {
		Logger.Info("RET_MODE: " + val)
		return //RET_MODE==just need the dummy ita endpoints
	}

	//make GCP_SA_CREDS, turkey-updater needs it to access turkeycfg bucket
	keyStr := os.Getenv("GCP_SA_KEY")
	f, _ := os.Create("/app/gcpkey.json")
	defer f.Close()
	f.WriteString(keyStr)
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/app/gcpkey.json")

	// prepare values
	cfg.SupportedChannels = []string{"dev", "beta", "stable"}

	cfg.PodNS = os.Getenv("POD_NS")
	if cfg.PodNS == "" {
		Logger.Error("POD_NS not set")
	}
	if strings.HasPrefix(cfg.PodNS, "hc-") {
		cfg.Tier = getEnv("TIER", "free")
	} else {
		cfg.Tier = "N/A"
	}
	Logger.Sugar().Infof("cfg.Tier: %v", cfg.Tier)
	cfg.Domain = os.Getenv("DOMAIN")
	cfg.RetApiKey = getEnv("RET_API_KEY", "probably not this")
	cfg.turkeyorchHost = getEnv("TURKEYORCH_HOST", "turkeyorch.turkey-services:889")

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

	cfg.RootUserEmail, _ = Get_fromNsAnnotations("adm")
	Logger.Sugar().Infof("cfg.RootUserEmail: %v", cfg.RootUserEmail)

	cfg.FreeTierIdleMax, err = time.ParseDuration(os.Getenv("FreeTierIdleMax"))
	if err != nil {
		Logger.Sugar().Warnf("failed to parse (FreeTierIdleMax): %v, falling back to default value", os.Getenv("FreeTierIdleMax"))
		cfg.FreeTierIdleMax = 30 * time.Minute
	}
	Logger.Sugar().Infof("cfg.turkeyorchHost: %v", cfg.turkeyorchHost)
	Logger.Sugar().Infof("cfg.FreeTierIdleMax: %v", cfg.FreeTierIdleMax)

	cfg.PodDeploymentName = getEnv("POD_DEPLOYMENT_NAME", "ita")
	cfg.ExtraHealthchecks = strings.Split(os.Getenv("EXTRA_HEALTHCHECKS"), ",")

	// features
	cfg.setFeatures()

	Logger.Sugar().Infof("cfg.Features: %+v", cfg.Features)

	if cfg.Features.customDomain {
		err = k8s_addItaApiIngressRule()
		if err != nil {
			Logger.Error(err.Error())
		}
		err := k8s_mountRetNfs("ita", "", "")
		if err != nil {
			Logger.Error(err.Error())
		}

		// err = k8s_mountRetNfs("hubs", "/hubs", "/www/hubs")
		// if err != nil {
		// 	Logger.Error(err.Error())
		// }

	}

	if cfg.Features.updater {
		cfg.Channel, err = Get_listeningChannelLabel()
		if err != nil {
			Logger.Warn("Get_listeningChannelLabel failed: " + err.Error())
			cfg.Channel = "unset"
			err := Set_listeningChannelLabel("unset")
			if err != nil {
				Logger.Error("Set_listeningChannelLabel failed: " + err.Error())
			}
		}
		cfg.TurkeyUpdater = NewTurkeyUpdater()
		err = cfg.TurkeyUpdater.Start(cfg.Channel)
		if err != nil {
			Logger.Error(err.Error())
		}
	}

}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

type itaFeatures struct {
	updater      bool
	customDomain bool
}

func New_itaFeatures() itaFeatures {
	return itaFeatures{
		updater:      false,
		customDomain: false,
	}
}

func (cfg *Config) setFeatures() {
	cfg.Features = New_itaFeatures()
	//turkey-updater
	if _, noUpdates := os.LookupEnv("NO_UPDATES"); !noUpdates {
		cfg.Features.updater = true
	}
	//custom-domain
	if slices.Contains([]string{
		"dev",
		"test",
	}, cfg.Tier) {
		cfg.Features.customDomain = true
	}

}
