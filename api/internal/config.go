package internal

import (
	"os"
)

type Config struct {
	Domain string `long:"domain" env:"DOMAIN" description:"turkey domain this k8s cluster's serving, example: myhubs.net"`
	DBconn string `long:"db-conn" env:"DB_CONN" description:"postgresql data base connection string"`
}

var Cfg *Config

func MakeCfg() {
	Cfg = &Config{}
	Cfg.DBconn = os.Getenv("DB_CONN")

}
