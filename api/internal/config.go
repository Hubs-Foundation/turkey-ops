package internal

import (
	"context"
	"net"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	Port        string
	PodIp       string
	PodNS       string
	PodLabelApp string

	Env      string `long:"environment" env:"ENV" description:"env config"`
	Domain   string `long:"domain" env:"DOMAIN" description:"turkey domain this k8s cluster's serving, example: myhubs.net"`
	DBuser   string `long:"db-user" env:"DB_USER" description:"postgresql data base username"`
	DBpass   string `long:"db-pass" env:"DB_PASS" description:"postgresql data base password"`
	DBconn   string `long:"db-conn" env:"DB_CONN" description:"postgresql data base connection string"`
	PermsKey string `long:"perms-key" env:"PERMS_KEY" description:"cluster wide private key for all reticulum authentications"`

	AwsKey    string `long:"aws-key" env:"AWS_KEY" description:"AWS_ACCESS_KEY_ID"`
	AwsSecret string `long:"aws-secret" env:"AWS_SECRET" description:"AWS_SECRET_ACCESS_KEY"`
	AwsRegion string `long:"aws-region" env:"AWS_REGION" description:"AWS_REGION"`

	SmtpServer string
	SmtpPort   string
	SmtpUser   string
	SmtpPass   string

	DockerhubUser string
	DockerhubPass string

	TurkeyCfg_s3_bkt  string
	DefaultRegion_aws string

	Awss       *AwsSvs
	K8ss_local *K8sSvs
}

var Cfg *Config

func MakeCfg() {
	Cfg = &Config{}

	Cfg.Port = getEnv("POD_PORT", "888")
	Cfg.PodIp = os.Getenv("POD_IP")
	if net.ParseIP(Cfg.PodIp) == nil {
		logger.Error("bad PodIp: " + Cfg.PodIp + ", things like groupcache will not work properly")
	}
	Cfg.PodNS = os.Getenv("POD_NS")
	if Cfg.PodNS == "" {
		logger.Error("missing env var: POD_NS")
	}
	Cfg.PodLabelApp = os.Getenv("POD_LABEL_APP")
	if Cfg.PodLabelApp == "" {
		logger.Error("missing env var: POD_LABEL_APP")
	}
	Cfg.Env = getEnv("ENV", "dev")
	Cfg.Domain = os.Getenv("DOMAIN")
	Cfg.DBconn = os.Getenv("DB_CONN")
	Cfg.DBuser = getEnv("DB_USER", "postgres")
	Cfg.DBpass = getEnv("DB_PASS", "throw")
	Cfg.AwsKey = os.Getenv("AWS_KEY")
	Cfg.AwsSecret = os.Getenv("AWS_SECRET")
	Cfg.AwsRegion = os.Getenv("AWS_REGION")
	Cfg.PermsKey = os.Getenv("PERMS_KEY")

	Cfg.SmtpServer = os.Getenv("SMTP_SERVER")
	Cfg.SmtpPort = os.Getenv("SMTP_PORT")
	Cfg.SmtpUser = os.Getenv("SMTP_USER")
	Cfg.SmtpPass = os.Getenv("SMTP_PASS")

	Cfg.DockerhubUser = os.Getenv("DOCKERHUB_USER")
	Cfg.DockerhubPass = os.Getenv("DOCKERHUB_PASS")

	Awss, err := NewAwsSvs(Cfg.AwsKey, Cfg.AwsSecret, Cfg.AwsRegion)
	if err != nil {
		GetLogger().Error("ERROR @ NewAwsSvs: " + err.Error())
	} else {
		accountNum, _ := Awss.GetAccountID()
		GetLogger().Info("aws acct#: " + accountNum)
	}
	Cfg.Awss = Awss

	Cfg.TurkeyCfg_s3_bkt = "turkeycfg"

	_ = os.Mkdir("./_files", os.ModePerm)
	f, _ := os.Create("./_files/ns_hc.yam")
	Awss.S3Download_file(Cfg.TurkeyCfg_s3_bkt, Cfg.Env+"/yams/ns_hc.yam", f)
	f.Close()

	Cfg.K8ss_local = NewK8sSvs_local()

	touchCfgMap("hubsbuilds-dev")
	touchCfgMap("hubsbuilds-beta")
	touchCfgMap("hubsbuilds-stable")

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

//////////////////////////

type HcNsNotes struct {
	Lastchecked time.Time
	Labels      map[string]string
}

var HcNsTable = map[string]HcNsNotes{}
