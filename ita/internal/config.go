package internal

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
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
	SubDomain         string
	HubDomain         string
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

	Features featureMan

	RootUserEmail string
	CustomDomain  string
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

	err := errors.New("dummy")

	cfg.RetApiKey = getEnv("RET_API_KEY", "probably not this")
	cfg.turkeyorchHost = getEnv("TURKEYORCH_HOST", "turkeyorch.turkey-services:888")

	cfg.HubDomain = getDomainFromOrch()
	if cfg.HubDomain == "" {
		Logger.Error("failed to getDomainFromOrch")
	}
	Logger.Info("cfg.HubDomain: " + cfg.HubDomain)

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

	cfg.SubDomain, err = NS_getLabel("subdomain")
	if err != nil {
		Logger.Error("failed to get subdomain with NS_getLabel: " + err.Error())
	}

	cfg.RootUserEmail, _ = Get_fromNsAnnotations("adm")
	Logger.Sugar().Infof("cfg.RootUserEmail: %v", cfg.RootUserEmail)
	cfg.CustomDomain, _ = Deployment_getLabel("custom-domain")

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
	cfg.Features = New_featureMan()
	cfg.Features.determineFeatures()
	Logger.Sugar().Infof("cfg.Features: %+v", cfg.Features.Get())
	cfg.Features.setupFeatures()

}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

///////////////////////////////////////////////////////////

type featureMan struct {
	_features hubFeatures
	mu        sync.Mutex
}

type hubFeatures struct {
	updater      bool
	customDomain bool
	customClient bool
}

func New_featureMan() featureMan {
	return featureMan{
		_features: hubFeatures{
			updater:      false,
			customDomain: false,
			customClient: false,
		},
	}
}

func (fm *featureMan) Get() hubFeatures {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	return fm._features
}

func (fm *featureMan) determineFeatures() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if slices.Contains([]string{"dev", "test"}, cfg.Tier) {
		fm._features.updater = true
		fm._features.customDomain = true
		fm._features.customClient = true
		return
	}

	if _, noUpdates := os.LookupEnv("NO_UPDATES"); !noUpdates {
		fm._features.updater = true

	}

	if slices.Contains([]string{"pro", "business"}, cfg.Tier) {
		fm._features.customDomain = true
	}

	customDomain, _ := Deployment_getLabel("custom-domain")
	if customDomain != "" {
		fm._features.customClient = true
	}
}

func (fm *featureMan) enableCustomClient() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm._features.customClient = true
}

func (fm *featureMan) setupFeatures() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	Logger.Sugar().Infof("initFeatures -- cfg.Features: %+v", fm._features)

	if fm._features.updater {
		cfg.TurkeyUpdater = NewTurkeyUpdater()
		err := cfg.TurkeyUpdater.Start()
		if err != nil {
			Logger.Error(err.Error())
		}
	}

	if fm._features.customClient {
		err := ingress_addItaApiRule()
		if err != nil {
			Logger.Error(err.Error())
		}
		err = k8s_mountRetNfs("ita", "", "")
		if err != nil {
			Logger.Error(err.Error())
		}
	}
}

/////////////////////////////////////////////////////////////

func getDomainFromOrch() string {
	resp, err := http.Get("http://" + cfg.turkeyorchHost + "/hub_domain")
	if err != nil {
		Logger.Error("err@getDomainFromOrch: " + err.Error())
		return ""
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		Logger.Error("err@getDomainFromOrch: " + err.Error())
		return ""
	}
	return string(respBytes)

}
