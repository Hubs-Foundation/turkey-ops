package idp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

// Fxa provider
type Fxa struct {
	ClientID     string `long:"client-id" env:"CLIENT_ID" description:"Client ID"`
	ClientSecret string `long:"client-secret" env:"CLIENT_SECRET" description:"Client Secret" json:"-"`
	Scope        string
	entrypoint   string
	Prompt       string `long:"prompt" env:"PROMPT" default:"select_account" description:"Space separated list of OpenID prompt options"`

	LoginURL *url.URL
	TokenURL *url.URL
	UserURL  *url.URL
}

// Name returns the name of the provider
func (f *Fxa) Name() string {
	return "fxa"
}

// Setup performs validation and setup
func (f *Fxa) Setup() error {
	if f.ClientID == "" || f.ClientSecret == "" {
		return errors.New("providers.fxa.client-id, providers.fxa.client-secret must be set")
	}

	// //default to staging fxa \
	// fxaLoginHost := "accounts.stage.mozaws.net"
	// fxaTokenHost := "oauth.stage.mozaws.net"
	// fxaUserHost := "profile.stage.mozaws.net"
	// //	unless it's a prod env
	// if os.Getenv("ENV") == "prod" {
	// 	fxaLoginHost = "accounts.firefox.com"
	// 	fxaTokenHost = "api.accounts.firefox.com"
	// 	fxaUserHost = "profile.accounts.firefox.com"
	// }

	fxaLoginHost := "accounts.firefox.com"
	fxaTokenHost := "api.accounts.firefox.com"
	fxaUserHost := "profile.accounts.firefox.com"

	// fxaLoginHost := os.Getenv("FXA_LOGIN_HOST")
	// if fxaLoginHost == "" {
	// 	fmt.Println("unset: FXA_LOGIN_HOST, falling back to accounts.stage.mozaws.net")
	// 	fxaLoginHost = "accounts.stage.mozaws.net"
	// }
	// fxaTokenHost := os.Getenv("FXA_TOKEN_HOST")
	// if fxaTokenHost == "" {
	// 	fmt.Println("unset: FXA_LOGIN_HOST, falling back to oauth.stage.mozaws.net")
	// 	fxaTokenHost = "oauth.stage.mozaws.net"
	// }
	// fxaUserHost := os.Getenv("FXA_USER_HOST")
	// if fxaUserHost == "" {
	// 	fmt.Println("unset: FXA_LOGIN_HOST, falling back to profile.stage.mozaws.net")
	// 	fxaUserHost = "profile.stage.mozaws.net"
	// }
	// Set static values
	f.Scope = "profile openid"
	f.LoginURL = &url.URL{
		Scheme: "https",
		Host:   fxaLoginHost,
		Path:   "/authorization",
	}
	f.TokenURL = &url.URL{
		Scheme: "https",
		Host:   fxaTokenHost,
		Path:   "/v1/token",
	}
	f.UserURL = &url.URL{
		Scheme: "https",
		Host:   fxaUserHost,
		Path:   "/v1/profile",
	}

	return nil
}

// GetLoginURL provides the login url for the given redirect uri and state
func (f *Fxa) GetLoginURL(redirectURI, state string) string {
	q := url.Values{}
	q.Set("client_id", f.ClientID)
	q.Set("scope", f.Scope)
	q.Set("entrypoint", redirectURI) // Todo could this be generated ad hoc by the client?
	q.Set("state", state)

	var u url.URL
	u = *f.LoginURL
	u.RawQuery = q.Encode()

	return u.String()
}

// ExchangeCode exchanges the given redirect uri and code for a token
func (f *Fxa) ExchangeCode(redirectURI, code string) (Token, error) {
	var token Token

	form := url.Values{}
	form.Set("client_id", f.ClientID)
	form.Set("client_secret", f.ClientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)
	form.Set("code", code)

	res, err := http.PostForm(f.TokenURL.String(), form)
	if err != nil {
		return token, err
	}

	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&token)

	if token.AccessToken == "" {
		return token, errors.New("failed to get token , res.StatusCode: " + strconv.Itoa(res.StatusCode) +
			", f.ClientID: " + f.ClientID[:4] + "..." + f.ClientID[len(f.ClientID)-4:] +
			", f.ClientSecret: " + f.ClientSecret[:4] + "..." + f.ClientSecret[len(f.ClientSecret)-4:] +
			", redirect_uri: " + redirectURI + ", code: " + code)
	}

	return token, err
}

// GetUser uses the given token and returns a complete provider.User object
func (f *Fxa) GetUser(token string) (User, error) {
	var user User

	client := &http.Client{}
	req, err := http.NewRequest("GET", f.UserURL.String(), nil)
	if err != nil {
		return user, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	res, err := client.Do(req)
	if err != nil {
		return user, err
	}
	defer res.Body.Close()
	fmt.Println("GetUser.res.StatusCode = ", res.StatusCode)

	// err = json.NewDecoder(res.Body).Decode(&user)
	bodyBytes, _ := ioutil.ReadAll(res.Body)
	fmt.Println("GetUser -- bodyBytes -- " + string(bodyBytes))
	err = json.Unmarshal(bodyBytes, &user)

	return user, err
}
