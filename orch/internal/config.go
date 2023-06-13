package internal

import (
	"context"
	"net"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	Port         string
	PodIp        string
	PodNS        string
	PodLabelApp  string
	AuthProxyUrl string `description:"auth proxy needs to produce http200 and various X-Forwarded headers for auth success (ie. X-Forwarded-UserEmail)"`

	Region                    string
	Env                       string `long:"environment" env:"ENV" description:"env name, used to select tf template file"`
	TurkeyJobsPubSubSubName   string
	TurkeyJobsPubSubTopicName string
	LAZY                      bool   `description:"Nack all jobs"`
	Channel                   string `long:"channel" env:"CHANNEL" description:"channel name, used to select turkey build channel"`
	Domain                    string `long:"domain" env:"DOMAIN" description:"example: myhubs.dev, this is the domain for turkey services, ie. asset and stream "`
	HubDomain                 string `long:"hubdomain" env:"HUB_DOMAIN" description:"example: myhubs.net, this is the domain for reticulum"`
	ClusterName               string
	DBuser                    string `long:"db-user" env:"DB_USER" description:"postgresql data base username"`
	DBpass                    string `long:"db-pass" env:"DB_PASS" description:"postgresql data base password"`
	DBconn                    string `long:"db-conn" env:"DB_CONN" description:"postgresql data base connection string"`
	PermsKey                  string `long:"perms-key" env:"PERMS_KEY" description:"cluster wide private key for all reticulum authentications"`
	FilestoreIP               string ``
	FilestorePath             string ``

	AwsKey               string `long:"aws-key" env:"AWS_KEY" description:"AWS_ACCESS_KEY_ID"`
	AwsSecret            string `long:"aws-secret" env:"AWS_SECRET" description:"AWS_SECRET_ACCESS_KEY"`
	AwsRegion            string `long:"aws-region" env:"AWS_REGION" description:"AWS_REGION"`
	GCP_SA_HMAC_KEY      string `description:"https://cloud.google.com/storage/docs/authentication/hmackeys, ie.0EWCp6g4j+MXn32RzOZ8eugSS5c0fydT88888888"`
	GCP_SA_HMAC_SECRET   string `description:"https://cloud.google.com/storage/docs/authentication/hmackeys, ie.0EWCp6g4j+MXn32RzOZ8eugSS5c0fydT88888888"`
	SmtpServer           string
	SmtpPort             string
	SmtpUser             string
	SmtpPass             string
	DASHBOARD_ACCESS_KEY string `description:"api key for turkey DASHBOARD access"`

	SKETCHFAB_API_KEY  string `description:"enables reticulum's sketchfab option"`
	TENOR_API_KEY      string `description "enables reticulum's gif option"`
	SENTRY_DSN_RET     string
	SENTRY_DSN_HUBS    string
	SENTRY_DSN_SPOKE   string
	HC_INIT_ASSET_PACK string `pre-installed asset pack for new hub instance`

	DockerhubUser string
	DockerhubPass string

	TurkeyCfg_s3_bkt  string
	DefaultRegion_aws string

	Awss       *AwsSvs
	Gcps       *GcpSvs
	K8ss_local *K8sSvs

	ImgRepo string
}

var Cfg *Config

func MakeCfg() {
	Cfg = &Config{}

	Cfg.Region = getEnv("REGION", "us-central1")

	Cfg.ImgRepo = "mozillareality"

	Cfg.AuthProxyUrl = os.Getenv("AUTH_PROXY_URL")

	Cfg.Port = getEnv("POD_PORT", "888")
	Cfg.PodIp = os.Getenv("POD_IP")
	if net.ParseIP(Cfg.PodIp) == nil {
		Logger.Error("bad PodIp: " + Cfg.PodIp + ", things like groupcache will not work properly")
	}
	Cfg.PodNS = os.Getenv("POD_NS")
	if Cfg.PodNS == "" {
		Logger.Error("missing env var: POD_NS")
	}
	Cfg.PodLabelApp = os.Getenv("POD_LABEL_APP")
	if Cfg.PodLabelApp == "" {
		Logger.Error("missing env var: POD_LABEL_APP")
	}
	Cfg.Env = getEnv("ENV", "dev")
	Logger.Info("Cfg.Env: " + Cfg.Env)

	Cfg.Channel = getEnv("CHANNEL", Cfg.Env)
	if Cfg.Env == "staging" {
		Cfg.Channel = "beta"
	} else if Cfg.Env == "prod" {
		Cfg.Channel = "stable"
	}

	Cfg.TurkeyJobsPubSubTopicName = "turkey_jobs"
	Cfg.TurkeyJobsPubSubSubName = "turkey_jobs_sub"
	if Cfg.Env == "dev" {
		Cfg.TurkeyJobsPubSubTopicName = "dev_turkey_jobs"
		Cfg.TurkeyJobsPubSubSubName = "dev_turkey_jobs_sub"
	}

	Cfg.LAZY = false
	if os.Getenv("LAZY") != "" {
		Cfg.LAZY = true
	}

	Logger.Info("Cfg.Channel: " + Cfg.Channel)
	Cfg.Domain = os.Getenv("DOMAIN")
	Cfg.HubDomain = os.Getenv("HUB_DOMAIN")

	Cfg.ClusterName = strings.Split(Cfg.HubDomain, ".")[0]
	Logger.Info("Cfg.ClusterName: " + Cfg.ClusterName)

	Cfg.DBconn = os.Getenv("DB_CONN")
	Cfg.FilestoreIP = os.Getenv("FilestoreIP")
	Cfg.FilestorePath = getEnv("FilestorePath", "vol1")
	Cfg.DBuser = getEnv("DB_USER", "postgres")
	Cfg.DBpass = getEnv("DB_PASS", "throw")
	Cfg.AwsKey = os.Getenv("AWS_KEY")
	Cfg.AwsSecret = os.Getenv("AWS_SECRET")
	Cfg.AwsRegion = os.Getenv("AWS_REGION")
	Cfg.GCP_SA_HMAC_KEY = os.Getenv("GCP_SA_HMAC_KEY")
	Cfg.GCP_SA_HMAC_SECRET = os.Getenv("GCP_SA_HMAC_SECRET")
	Cfg.DASHBOARD_ACCESS_KEY = getEnv("DASHBOARD_ACCESS_KEY", "dummy_P@$$")

	Cfg.PermsKey = os.Getenv("PERMS_KEY")

	Cfg.SmtpServer = os.Getenv("SMTP_SERVER")
	Cfg.SmtpPort = os.Getenv("SMTP_PORT")
	Cfg.SmtpUser = os.Getenv("SMTP_USER")
	Cfg.SmtpPass = os.Getenv("SMTP_PASS")

	Cfg.SKETCHFAB_API_KEY = os.Getenv("SKETCHFAB_API_KEY")
	Cfg.TENOR_API_KEY = os.Getenv("TENOR_API_KEY")
	Cfg.SENTRY_DSN_RET = os.Getenv("SENTRY_DSN_RET")
	Cfg.SENTRY_DSN_HUBS = os.Getenv("SENTRY_DSN_HUBS")
	Cfg.SENTRY_DSN_SPOKE = os.Getenv("SENTRY_DSN_SPOKE")
	Cfg.HC_INIT_ASSET_PACK = getEnv("HC_INIT_ASSET_PACK", "https://raw.githubusercontent.com/mozilla/hubs-cloud/master/asset-packs/turkey-init.pack")

	Cfg.DockerhubUser = os.Getenv("DOCKERHUB_USER")
	Cfg.DockerhubPass = os.Getenv("DOCKERHUB_PASS")

	Cfg.Awss = makeAwss()
	Cfg.Gcps = makeGcpSvs()

	Cfg.TurkeyCfg_s3_bkt = "turkeycfg"

	// _ = os.Mkdir("./_files", os.ModePerm)
	// f, _ := os.Create("./_files/ns_hc.yam")
	// Cfg.Awss.S3Download_file(Cfg.TurkeyCfg_s3_bkt, Cfg.Env+"/yams/ns_hc.yam", f)
	// f.Close()

	Cfg.K8ss_local = NewK8sSvs_local()
	if Cfg.K8ss_local != nil {

		touchCfgMap("hubsbuilds-dev")
		touchCfgMap("hubsbuilds-beta")
		touchCfgMap("hubsbuilds-stable")
	}

}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func touchCfgMap(name string) error {
	_, err := Cfg.K8ss_local.ClientSet.CoreV1().ConfigMaps(Cfg.PodNS).
		Create(
			context.Background(),
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"foo": "bar"}}},
			metav1.CreateOptions{},
		)
	return err
}

func makeAwss() *AwsSvs {
	Awss, err := NewAwsSvs(Cfg.AwsKey, Cfg.AwsSecret, Cfg.AwsRegion)
	if err != nil {
		GetLogger().Error("ERROR @ NewAwsSvs: " + err.Error())
	} else {
		accountNum, _ := Awss.GetAccountID()
		GetLogger().Info("aws acct#: " + accountNum)
	}
	return Awss
}

func makeGcpSvs() *GcpSvs {

	keyStr := os.Getenv("GCP_SA_KEY")
	f, _ := os.Create("/app/gcpkey.json")
	defer f.Close()
	f.WriteString(keyStr)
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/app/gcpkey.json")
	Gcps, err := NewGcpSvs()
	if err != nil {
		GetLogger().Error("ERROR @ NewGcpSvs: " + err.Error())
	} else {
		GetLogger().Info("gcp project id: " + Gcps.ProjectId)
	}
	return Gcps
}

//////////////////////////
