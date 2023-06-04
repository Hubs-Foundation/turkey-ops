package internal

import "os"

type Config struct {
	HubDomain string `cluster's HubDomain`
	Domain    string `cluster's Domain`
}

var Cfg *Config

func MakeCfg() {
	Cfg = &Config{}

	Cfg.Domain = getEnv("Domain", "myhubs.dev")
	Cfg.HubDomain = getEnv("HubDomain", "myhubs.net")

}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
