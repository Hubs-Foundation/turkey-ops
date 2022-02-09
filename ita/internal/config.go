package internal

import (
	"context"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var cfg *Config

// Config holds the runtime application config
type Config struct {
	PodNS             string
	PodDeploymentName string
	Domain            string `turkey domain`

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
	var err error
	cfg.K8sCfg, err = rest.InClusterConfig()
	if err != nil {
		Logger.Error(err.Error())
	}
	cfg.K8sClientSet, err = kubernetes.NewForConfig(cfg.K8sCfg)
	if err != nil {
		Logger.Error(err.Error())
	}

	cfg.Domain = os.Getenv("DOMAIN")
	cfg.PodDeploymentName = getEnv("POD_DEPLOYMENT_NAME", "ita")
	cfg.PodNS = os.Getenv("POD_NS")
	if cfg.PodNS == "" {
		Logger.Error("POD_NS not set")
	}

	//do we have channel labled on deployment?
	d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), cfg.PodDeploymentName, metav1.GetOptions{})
	if err != nil {
		Logger.Error("failed to get local deployment: " + cfg.PodDeploymentName)
	}
	cfg.ListeningChannel = d.Labels["CHANNEL"]
	//unexpected(or empty) channel value ==> fallback to stable
	_, ok := cfg.SupportedChannels[cfg.ListeningChannel]
	if !ok {
		Logger.Warn("bad env var CHANNEL: " + cfg.ListeningChannel + ", so we'll use stable")
		cfg.ListeningChannel = "stable"
	}

	//need channel on ***ita deployment's*** label -- if not already
	if d.Labels == nil || d.Labels["CHANNEL"] != cfg.ListeningChannel {
		if d.Labels == nil {
			d.Labels = make(map[string]string)
		}
		d.Labels["CHANNEL"] = cfg.ListeningChannel
		_, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), d, metav1.UpdateOptions{})
		if err != nil {
			Logger.Error("failed to update channel to deployment.labels: " + err.Error())
		}
	}

	cfg.TurkeyUpdater = NewTurkeyUpdater()
	_, err = cfg.TurkeyUpdater.Start()
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
