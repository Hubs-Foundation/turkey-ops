package internal

import (
	"crypto"
	"crypto/rsa"
	"encoding/json"
	"os"
	"strings"
	"time"
)

var cfg *Config

// Config holds the runtime application config
type Config struct {
	Env            string `description:"dev/staging/prod"`
	TurkeyDomain   string `description:"turkey domain"`
	Domain         string `description:"root domain"`
	TrustedClients string `description:"ie. https://portal.myhubs.net"`

	AuthHost               string               `long:"auth-host" env:"AUTH_HOST" description:"Single host to use when returning from 3rd party auth"`
	Config                 func(s string) error `long:"config" env:"CONFIG" description:"Path to config file" json:"-"`
	InsecureCookie         bool                 `long:"insecure-cookie" env:"INSECURE_COOKIE" description:"Use insecure cookies"`
	CookieName             string               `long:"cookie-name" env:"COOKIE_NAME" default:"_forward_auth" description:"Cookie Name"`
	JwtCookieName          string               `long:"cookie-name" env:"COOKIE_NAME" default:"_forward_auth" description:"Cookie Name"`
	CSRFCookieName         string               `long:"csrf-cookie-name" env:"CSRF_COOKIE_NAME" default:"_forward_auth_csrf" description:"CSRF Cookie Name"`
	DefaultAction          string               `long:"default-action" env:"DEFAULT_ACTION" default:"auth" choice:"auth,allow" description:"Default action"`
	DefaultProvider        string               `long:"default-provider" env:"DEFAULT_PROVIDER" default:"fxa" choice:"google,fxa,oidc,generic-oauth" description:"Default provider"`
	LifetimeString         int                  `long:"lifetime" env:"LIFETIME" default:"43200" description:"Lifetime in seconds"`
	LogoutRedirect         string               `long:"logout-redirect" env:"LOGOUT_REDIRECT" description:"URL to redirect to following logout"`
	MatchWhitelistOrDomain bool                 `long:"match-whitelist-or-domain" env:"MATCH_WHITELIST_OR_DOMAIN" description:"Allow users that match *either* whitelist or domain (enabled by default in v3)"`
	Path                   string               `long:"url-path" env:"URL_PATH" default:"/_oauth" description:"Callback URL Path"`
	CookieSecret           string               `long:"secret" env:"COOKIE_SECRET" description:"Secret used for signing auth cookies (required)" json:"-"`
	Port                   int                  `long:"port" env:"PORT" default:"4181" description:"Port to listen on"`

	// Filled during transformations
	Secret   []byte `json:"-"`
	Lifetime time.Duration

	PermsKey     *rsa.PrivateKey  `description:"cluster wide private key for all reticulum authentications ... used to sign jwt tokens here"`
	PermsKey_pub crypto.PublicKey `description:"public part of PermsKey ... used to verify jwt tokens here"`
}

func MakeCfg() {
	cfg = &Config{}

	cfg.Env = os.Getenv("ENV")
	if cfg.Env == "" {
		cfg.Env = "dev"
	}

	cfg.TurkeyDomain = os.Getenv("turkeyDomain")
	rootDomain := rootDomain(cfg.TurkeyDomain)
	if rootDomain == "" {
		Logger.Error("bad turkeyDomain env var: <" + cfg.TurkeyDomain + "> falling back to <myhubs.net>")
		rootDomain = "myhubs.net"
	}

}

func rootDomain(fullDomain string) string {
	fdArr := strings.Split(fullDomain, ".")
	len := len(fdArr)
	if len < 2 {
		return ""
	}
	return fdArr[len-2] + "." + fdArr[len-1]
}

func (c Config) String() string {
	jsonConf, _ := json.Marshal(c)
	return string(jsonConf)
}
