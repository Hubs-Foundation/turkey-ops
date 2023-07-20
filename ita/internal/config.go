package internal

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	K8Man    *k8Man

	RootUserEmail       string
	CustomDomain        string
	CustomDomainCertExp time.Time

	Ret_guardiankey string
	Ret_phxkey      string
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
		cfg.Tier = getEnv("TIER", "p0")
	} else {
		cfg.Tier = "N/A"
	}
	Logger.Sugar().Infof("cfg.Tier: %v", cfg.Tier)

	err := errors.New("dummy")

	cfg.RetApiKey = getEnv("RET_API_KEY", "probably not this")
	cfg.turkeyorchHost = getEnv("TURKEYORCH_HOST", "turkeyorch.turkey-services:888")

	cfg.HubDomain = getDomainFromOrch()
	if cfg.HubDomain == "" {
		Logger.Fatal("failed to getDomainFromOrch")
	}
	Logger.Info("cfg.HubDomain: " + cfg.HubDomain)

	Hostname, err := os.Hostname()
	if err != nil {
		Logger.Warn("failed to get Hostname")
	} else {
		Logger.Debug("Hostname: " + Hostname)
	}
	cfg.HostnameHash = hash(Hostname)

	cfg.K8Man = New_k8Man()

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
		Logger.Fatal("failed to get subdomain with NS_getLabel: " + err.Error())
	}
	Logger.Info("cfg.SubDomain: " + cfg.SubDomain)

	cfg.RootUserEmail, err = Get_fromNsAnnotations("adm")
	if err != nil {
		Logger.Fatal("failed @Get_fromNsAnnotations: " + err.Error())
	}
	Logger.Info("cfg.RootUserEmail: " + cfg.RootUserEmail)

	Logger.Sugar().Info("cfg.RootUserEmail: %v", cfg.RootUserEmail)

	cfg.Ret_guardiankey, cfg.Ret_phxkey = GetRetKeys()
	Logger.Sugar().Info("cfg.Ret_guardiankey: %v, cfg.Ret_phxkey: %v",
		cfg.Ret_guardiankey, cfg.Ret_phxkey)

	cfg.FreeTierIdleMax, err = time.ParseDuration(os.Getenv("FreeTierIdleMax"))
	if err != nil {
		Logger.Sugar().Warnf("failed to parse (FreeTierIdleMax): %v, falling back to default value", os.Getenv("FreeTierIdleMax"))
		cfg.FreeTierIdleMax = 12 * time.Hour
		if cfg.Tier == "p1" {
			cfg.FreeTierIdleMax = 72 * time.Hour
		}
		if strings.HasSuffix(cfg.HubDomain, "dev.myhubs.net") {
			cfg.FreeTierIdleMax = 15 * time.Minute
		}
	}
	Logger.Sugar().Infof("cfg.turkeyorchHost: %v", cfg.turkeyorchHost)
	Logger.Sugar().Infof("cfg.FreeTierIdleMax: %v", cfg.FreeTierIdleMax)

	cfg.PodDeploymentName = getEnv("POD_DEPLOYMENT_NAME", "ita")

	cfg.CustomDomain, _ = Deployment_getLabel("custom-domain")
	Logger.Sugar().Infof("cfg.CustomDomain: %v", cfg.CustomDomain)

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
