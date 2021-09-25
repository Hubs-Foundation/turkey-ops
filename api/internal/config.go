package internal

import (
	"os"
)

type Config struct {
	DBconn string `long:"db-conn" env:"DB_CONN" description:"postgresql data base connection string"`
}

var cfg *Config

func MakeCfg() {
	cfg = &Config{}
	cfg.DBconn = os.Getenv("DB_CONN")

}
