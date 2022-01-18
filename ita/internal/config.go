package internal

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var cfg *Config

// Config holds the runtime application config
type Config struct {
	Domain string `turkey domain`

	K8sCfg       *rest.Config
	K8sNamespace string
	K8sClientSet *kubernetes.Clientset

	// TrustedClients string `ie. https://portal.myhubs.net`

	// AuthHost               string               `long:"auth-host" env:"AUTH_HOST" description:"Single host to use when returning from 3rd party auth"`
	// Config                 func(s string) error `long:"config" env:"CONFIG" description:"Path to config file" json:"-"`
	// InsecureCookie         bool                 `long:"insecure-cookie" env:"INSECURE_COOKIE" description:"Use insecure cookies"`
	// CookieName             string               `long:"cookie-name" env:"COOKIE_NAME" default:"_forward_auth" description:"Cookie Name"`
	// CSRFCookieName         string               `long:"csrf-cookie-name" env:"CSRF_COOKIE_NAME" default:"_forward_auth_csrf" description:"CSRF Cookie Name"`
	// DefaultAction          string               `long:"default-action" env:"DEFAULT_ACTION" default:"auth" choice:"auth" choice:"allow" description:"Default action"`
	// DefaultProvider        string               `long:"default-provider" env:"DEFAULT_PROVIDER" default:"google" choice:"google" choice:"oidc" choice:"generic-oauth" description:"Default provider"`
	// LifetimeString         int                  `long:"lifetime" env:"LIFETIME" default:"43200" description:"Lifetime in seconds"`
	// LogoutRedirect         string               `long:"logout-redirect" env:"LOGOUT_REDIRECT" description:"URL to redirect to following logout"`
	// MatchWhitelistOrDomain bool                 `long:"match-whitelist-or-domain" env:"MATCH_WHITELIST_OR_DOMAIN" description:"Allow users that match *either* whitelist or domain (enabled by default in v3)"`
	// Path                   string               `long:"url-path" env:"URL_PATH" default:"/_oauth" description:"Callback URL Path"`
	// CookieSecret           string               `long:"secret" env:"COOKIE_SECRET" description:"Secret used for signing auth cookies (required)" json:"-"`
	// Port                   int                  `long:"port" env:"PORT" default:"4181" description:"Port to listen on"`

	// Providers idp.Providers    `group:"providers" namespace:"providers" env-namespace:"PROVIDERS"`
	// Rules     map[string]*Rule `long:"rule.<name>.<param>" description:"Rule definitions, param can be: \"action\", \"rule\" or \"provider\""`

	// // Filled during transformations
	// Secret   []byte `json:"-"`
	// Lifetime time.Duration

}

func MakeCfg() {
	cfg = &Config{}
	cfg.Domain = os.Getenv("DOMAIN")

	var err error

	cfg.K8sCfg, err = rest.InClusterConfig()
	if err != nil {
		Logger.Error(err.Error())
	}

	cfg.K8sNamespace = os.Getenv("POD_NS")
	if cfg.K8sNamespace == "" {
		Logger.Error("POD_NS not set")
	}
	cfg.K8sClientSet, err = kubernetes.NewForConfig(cfg.K8sCfg)
	if err != nil {
		Logger.Error(err.Error())
	}

}
