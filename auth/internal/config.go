package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"main/internal/idp"
)

var cfg *Config

// Config holds the runtime application config
type Config struct {
	TurkeyDomain   string `turkey domain`
	Domain         string `root domain`
	TrustedClients string `ie. https://portal.myhubs.net`

	AuthHost               string               `long:"auth-host" env:"AUTH_HOST" description:"Single host to use when returning from 3rd party auth"`
	Config                 func(s string) error `long:"config" env:"CONFIG" description:"Path to config file" json:"-"`
	CookieDomains          []CookieDomain       `long:"cookie-domain" env:"COOKIE_DOMAIN" env-delim:"," description:"Domain to set auth cookie on, can be set multiple times"`
	InsecureCookie         bool                 `long:"insecure-cookie" env:"INSECURE_COOKIE" description:"Use insecure cookies"`
	CookieName             string               `long:"cookie-name" env:"COOKIE_NAME" default:"_forward_auth" description:"Cookie Name"`
	CSRFCookieName         string               `long:"csrf-cookie-name" env:"CSRF_COOKIE_NAME" default:"_forward_auth_csrf" description:"CSRF Cookie Name"`
	DefaultAction          string               `long:"default-action" env:"DEFAULT_ACTION" default:"auth" choice:"auth" choice:"allow" description:"Default action"`
	DefaultProvider        string               `long:"default-provider" env:"DEFAULT_PROVIDER" default:"google" choice:"google" choice:"oidc" choice:"generic-oauth" description:"Default provider"`
	Domains                CommaSeparatedList   `long:"domain" env:"DOMAIN" env-delim:"," description:"Only allow given email domains, can be set multiple times"`
	LifetimeString         int                  `long:"lifetime" env:"LIFETIME" default:"43200" description:"Lifetime in seconds"`
	LogoutRedirect         string               `long:"logout-redirect" env:"LOGOUT_REDIRECT" description:"URL to redirect to following logout"`
	MatchWhitelistOrDomain bool                 `long:"match-whitelist-or-domain" env:"MATCH_WHITELIST_OR_DOMAIN" description:"Allow users that match *either* whitelist or domain (enabled by default in v3)"`
	Path                   string               `long:"url-path" env:"URL_PATH" default:"/_oauth" description:"Callback URL Path"`
	CookieSecret           string               `long:"secret" env:"COOKIE_SECRET" description:"Secret used for signing auth cookies (required)" json:"-"`
	Whitelist              CommaSeparatedList   `long:"whitelist" env:"WHITELIST" env-delim:"," description:"Only allow given email addresses, can be set multiple times"`
	Port                   int                  `long:"port" env:"PORT" default:"4181" description:"Port to listen on"`

	Providers idp.Providers    `group:"providers" namespace:"providers" env-namespace:"PROVIDERS"`
	Rules     map[string]*Rule `long:"rule.<name>.<param>" description:"Rule definitions, param can be: \"action\", \"rule\" or \"provider\""`

	// Filled during transformations
	Secret   []byte `json:"-"`
	Lifetime time.Duration
}

func MakeCfg() {
	cfg = &Config{}
	cfg.TurkeyDomain = os.Getenv("turkeyDomain")
	rootDomain := rootDomain(cfg.TurkeyDomain)
	if rootDomain == "" {
		Logger.Error("bad turkeyDomain env var: " + cfg.TurkeyDomain + "falling back to <myhubs.net>")
		rootDomain = "myhubs.net"
	}
	cfg.Domain = rootDomain
	cfg.AuthHost = rootDomain
	cfg.TrustedClients = os.Getenv("trustedClients")
	cfg.Secret = []byte("dummy-SecretString-replace-me-with-env-var-later")
	cfg.Lifetime = time.Second * time.Duration(43200) //12 hours
	cfg.CookieName = "_turkeyauthcookie"
	cfg.CSRFCookieName = "_turkeyauthcsrfcookie"
	cfg.CookieDomains = []CookieDomain{*NewCookieDomain(rootDomain)}
	// cfg.LogoutRedirect = "https://api." + cfg.TurkeyDomain + "/console"
	cfg.LogoutRedirect = "https://hubs.mozilla.com"

	cfg.DefaultProvider = "fxa"
	// GOOGLE
	cfg.Providers.Google.ClientID = os.Getenv("oauthClientId_google")
	cfg.Providers.Google.ClientSecret = os.Getenv("oauthClientSecret_google")
	err := cfg.Providers.Google.Setup()

	if err != nil {
		Logger.Error("[ERROR] @ Cfg.Providers.Google.Setup: " + err.Error())
	}

	// FXA
	cfg.Providers.Fxa.ClientID = os.Getenv("oauthClientId_fxa")
	cfg.Providers.Fxa.ClientSecret = os.Getenv("oauthClientSecret_fxa")
	err = cfg.Providers.Fxa.Setup()

	if err != nil {
		Logger.Error("[ERROR] @ Cfg.Providers.Fxa.Setup: " + err.Error())
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

// Validate validates a config object
func (c *Config) Validate() {
	// Check for show stopper errors
	if len(c.Secret) == 0 {
		Logger.Fatal("\"secret\" option must be set")
	}

	// Setup default provider
	err := c.setupProvider(c.DefaultProvider)
	if err != nil {
		Logger.Fatal(err.Error())
	}

	// Check rules (validates the rule and the rule provider)
	for _, rule := range c.Rules {
		err = rule.Validate(c)
		if err != nil {
			Logger.Fatal(err.Error())
		}
	}
}

func (c Config) String() string {
	jsonConf, _ := json.Marshal(c)
	return string(jsonConf)
}

// GetProvider returns the provider of the given name
func (c *Config) GetProvider(name string) (idp.Provider, error) {
	switch name {
	case "google":
		return &c.Providers.Google, nil
		// case "oidc":
		// 	return &c.Providers.OIDC, nil
		// case "generic-oauth":
		// 	return &c.Providers.GenericOAuth, nil
	case "fxa":
		Logger.Debug(" ### GetProvider: fxa")
		return &c.Providers.Fxa, nil
	}

	return nil, fmt.Errorf("unknown provider: %s", name)
}

// GetConfiguredProvider returns the provider of the given name, if it has been
// configured. Returns an error if the provider is unknown, or hasn't been configured
// func (c *Config) GetConfiguredProvider(name string) (provider.Provider, error) {
// 	// Check the provider has been configured
// 	if !c.providerConfigured(name) {
// 		return nil, fmt.Errorf("Unconfigured provider: %s", name)
// 	}

// 	return c.GetProvider(name)
// }

// func (c *Config) providerConfigured(name string) bool {
// 	// Check default provider
// 	if name == c.DefaultProvider {
// 		return true
// 	}

// 	// Check rule providers
// 	for _, rule := range c.Rules {
// 		if name == rule.Provider {
// 			return true
// 		}
// 	}

// 	return false
// }

func (c *Config) setupProvider(name string) error {
	// Check provider exists
	p, err := c.GetProvider(name)
	if err != nil {
		return err
	}

	// Setup
	err = p.Setup()
	if err != nil {
		return err
	}

	return nil
}

// Rule holds defined rules
type Rule struct {
	Action    string
	Rule      string
	Provider  string
	Whitelist CommaSeparatedList
	Domains   CommaSeparatedList
}

// NewRule creates a new rule object
func NewRule() *Rule {
	return &Rule{
		Action: "auth",
	}
}

func (r *Rule) formattedRule() string {
	// Traefik implements their own "Host" matcher and then offers "HostRegexp"
	// to invoke the mux "Host" matcher. This ensures the mux version is used
	return strings.ReplaceAll(r.Rule, "Host(", "HostRegexp(")
}

// Validate validates a rule
func (r *Rule) Validate(c *Config) error {
	if r.Action != "auth" && r.Action != "allow" {
		return errors.New("invalid rule action, must be \"auth\" or \"allow\"")
	}

	return c.setupProvider(r.Provider)
}

// Legacy support for comma separated lists

// CommaSeparatedList provides legacy support for config values provided as csv
type CommaSeparatedList []string

// UnmarshalFlag converts a comma separated list to an array
func (c *CommaSeparatedList) UnmarshalFlag(value string) error {
	*c = append(*c, strings.Split(value, ",")...)
	return nil
}

// MarshalFlag converts an array back to a comma separated list
func (c *CommaSeparatedList) MarshalFlag() (string, error) {
	return strings.Join(*c, ","), nil
}
