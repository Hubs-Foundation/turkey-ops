package idp

import (
	"context"
	// "net/url"

	"golang.org/x/oauth2"
)

// Providers contains all the implemented providers
type Providers struct {
	Google Google `group:"Google Provider" namespace:"google" env-namespace:"GOOGLE"`
	Fxa    Fxa    `group:"FxA Provider" namespace:"fxa" env-namespace:"FXA"`
	// OIDC         OIDC         `group:"OIDC Provider" namespace:"oidc" env-namespace:"OIDC"`
	GenericOAuth GenericOAuth `group:"Generic OAuth2 Provider" namespace:"generic-oauth" env-namespace:"GENERIC_OAUTH"`
}

// Provider is used to authenticate users
type Provider interface {
	Name() string
	GetLoginURL(redirectURI, state string) string
	ExchangeCode(redirectURI, code string) (Token, error)
	GetUser(token string) (User, error)
	GetSubscriptions(token string, user *User) error
	Setup() error
}

type Token struct {
	AccessToken string `json:"access_token"`
	IdToken     string `json:"id_token"`
}

// User is the authenticated user
type User struct {
	Email          string   `json:"email"`
	Locale         string   `json:"locale"`
	TwoFA          bool     `json:"twoFactorAuthentication"`
	MetricsEnabled bool     `json:"metricsEnabled"`
	Uid            string   `json:"uid"`
	Avatar         string   `json:"avatar"`
	AvatarDefault  bool     `json:"avatarDefault"`
	Sub            string   `json:"sub"`
	DisplayName    string   `json:"displayName"`
	Subscriptions  []string `json:"subscriptions"`
	//
	Plan_id              string
	Cancel_at_period_end bool
	Current_period_end   float64
}

// OAuthProvider is a provider using the oauth2 library
type OAuthProvider struct {
	Resource string `long:"resource" env:"RESOURCE" description:"Optional resource indicator"`

	Config *oauth2.Config
	ctx    context.Context
}

// ConfigCopy returns a copy of the oauth2 config with the given redirectURI
// which ensures the underlying config is not modified
func (p *OAuthProvider) ConfigCopy(redirectURI string) oauth2.Config {
	config := *p.Config
	config.RedirectURL = redirectURI
	return config
}

// OAuthGetLoginURL provides a base "GetLoginURL" for providers using OAauth2
func (p *OAuthProvider) OAuthGetLoginURL(redirectURI, state string) string {
	config := p.ConfigCopy(redirectURI)

	if p.Resource != "" {
		return config.AuthCodeURL(state, oauth2.SetAuthURLParam("resource", p.Resource))
	}

	return config.AuthCodeURL(state)
}

// OAuthExchangeCode provides a base "ExchangeCode" for providers using OAauth2
func (p *OAuthProvider) OAuthExchangeCode(redirectURI, code string) (*oauth2.Token, error) {
	config := p.ConfigCopy(redirectURI)
	return config.Exchange(p.ctx, code)
}
