package internal

import (
	"os"
)

type Config struct {
	Env      string `long:"environment" env:"ENV" description:"env config"`
	Domain   string `long:"domain" env:"DOMAIN" description:"turkey domain this k8s cluster's serving, example: myhubs.net"`
	DBuser   string `long:"db-user" env:"DB_USER" description:"postgresql data base user"`
	DBconn   string `long:"db-conn" env:"DB_CONN" description:"postgresql data base connection string"`
	PermsKey string `long:"perms-key" env:"PERMS_KEY" description:"cluster wide private key for all reticulum authentications"`

	AwsKey    string `long:"aws-key" env:"AWS_KEY" description:"AWS_ACCESS_KEY_ID"`
	AwsSecret string `long:"aws-secret" env:"AWS_SECRET" description:"AWS_SECRET_ACCESS_KEY"`
	AwsRegion string `long:"aws-region" env:"AWS_REGION" description:"AWS_REGION"`

	TurkeyCfg_s3_bkt  string
	DefaultRegion_aws string

	Awss *AwsSvs
}

var Cfg *Config

func MakeCfg() {
	Cfg = &Config{}
	Cfg.Env = getEnv("ENV", "dev")
	Cfg.Domain = os.Getenv("DOMAIN")
	Cfg.DBconn = os.Getenv("DB_CONN")
	Cfg.DBuser = getEnv("DB_USER", "postgres")
	Cfg.AwsKey = os.Getenv("AWS_KEY")
	Cfg.AwsSecret = os.Getenv("AWS_SECRET")
	Cfg.AwsRegion = os.Getenv("AWS_REGION")
	Cfg.PermsKey = os.Getenv("PERMS_KEY")

	Awss, err := NewAwsSvs(Cfg.AwsKey, Cfg.AwsSecret, Cfg.AwsRegion)
	if err != nil {
		GetLogger().Error("ERROR @ NewAwsSvs: " + err.Error())
	} else {
		accountNum, _ := Awss.GetAccountID()
		GetLogger().Info("aws acct#: " + accountNum)
	}
	Cfg.Awss = Awss

	Cfg.TurkeyCfg_s3_bkt = "turkeycfg"

	f, _ := os.Create("./_files/hc.yam")
	Awss.S3Download_file(Cfg.TurkeyCfg_s3_bkt, Cfg.Env+"/hc.yam", f)
	f.Close()

}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
