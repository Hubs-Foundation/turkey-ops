package internal

import (
	"os"
)

type Config struct {
	DBconn string `long:"db-conn" env:"DB_CONN" description:"postgresql data base connection string"`
}

var Cfg *Config

func MakeCfg() {
	Cfg = &Config{}
	Cfg.DBconn = os.Getenv("DB_CONN")

}
