package internal

import (
	"os"
)

type Config struct {
	Domain    string `long:"domain" env:"DOMAIN" description:"turkey domain this k8s cluster's serving, example: myhubs.net"`
	DBuser    string `long:"db-user" env:"DB_USER" description:"postgresql data base user"`
	DBconn    string `long:"db-conn" env:"DB_CONN" description:"postgresql data base connection string"`
	AwsKey    string `long:"aws-key" env:"AWS_KEY" description:"AWS_ACCESS_KEY_ID"`
	AwsSecret string `long:"aws-secret" env:"AWS_SECRET" description:"AWS_SECRET_ACCESS_KEY"`
	AwsRegion string `long:"aws-region" env:"AWS_REGION" description:"AWS_REGION"`

	Awss *AwsSvs
}

var Cfg *Config

func MakeCfg() {
	Cfg = &Config{}
	Cfg.Domain = os.Getenv("DOMAIN")
	Cfg.DBconn = os.Getenv("DB_CONN")
	Cfg.DBuser = "postgres"
	Cfg.AwsKey = os.Getenv("AWS_KEY")
	Cfg.AwsSecret = os.Getenv("AWS_SECRET")
	Cfg.AwsRegion = os.Getenv("AWS_REGION")

	Awss, err := NewAwsSvs(Cfg.AwsKey, Cfg.AwsSecret, Cfg.AwsRegion)
	if err != nil {
		GetLogger().Error("ERROR @ NewAwsSvs: " + err.Error())
	} else {
		accountNum, _ := Awss.GetAccountID()
		GetLogger().Info("aws acct#: " + accountNum)
	}
	Cfg.Awss = Awss

}
