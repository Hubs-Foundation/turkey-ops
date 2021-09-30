package idp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
func (g *Fxa) Name() string {
	return "fxa"
}

// Setup performs validation and setup
func (g *Fxa) Setup() error {
	if g.ClientID == "" || g.ClientSecret == "" {
		return errors.New("providers.fxa.client-id, providers.fxa.client-secret must be set")
	}

	// Set static values
	g.Scope = "profile openid"
	g.LoginURL = &url.URL{
		Scheme: "https",
		Host:   "localhost:3030",
		Path:   "/authorization",
	}
	g.TokenURL = &url.URL{
		Scheme: "https",
		Host:   "api-accounts.stage.mozaws.net",
		Path:   "/v1/client",
	}
	// g.UserURL = &url.URL{
	// 	Scheme: "https",
	// 	Host:   "localhost:3030",
	// 	Path:   "/oauth2/v2/userinfo",
	// }

	return nil
}

// GetLoginURL provides the login url for the given redirect uri and state
func (g *Fxa) GetLoginURL(redirectURI, state string) string {
	q := url.Values{}
	q.Set("client_id", g.ClientID)
	q.Set("scope", g.Scope)
	q.Set("entrypoint", g.entrypoint) // Todo could this be generated ad hoc by the client?
	q.Set("state", state)

	var u url.URL
	u = *g.LoginURL
	u.RawQuery = q.Encode()

	return u.String()
}

// ExchangeCode exchanges the given redirect uri and code for a token
func (g *Fxa) ExchangeCode(redirectURI, code string) (Token, error) {
	var token Token

	form := url.Values{}
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)
	form.Set("code", code)

	res, err := http.PostForm(g.TokenURL.String(), form)
	if err != nil {
		return token, err
	}

	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&token)
	// fmt.Sprintln("ExchangeCode.res.StatusCode = ", res.StatusCode)
	// bodyBytes, err := ioutil.ReadAll(res.Body)

	return token, err
}

// GetUser uses the given token and returns a complete provider.User object
func (g *Fxa) GetUser(token string) (User, error) {
	var user User

	client := &http.Client{}
	req, err := http.NewRequest("GET", g.UserURL.String(), nil)
	if err != nil {
		return user, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	res, err := client.Do(req)
	if err != nil {
		return user, err
	}
	defer res.Body.Close()
	fmt.Sprintln("GetUser.res.StatusCode = ", res.StatusCode)

	err = json.NewDecoder(res.Body).Decode(&user)
	// bodyBytes, err := ioutil.ReadAll(res.Body)
	// fmt.Println("GetUser -- bodyBytes -- " + string(bodyBytes))
	// err = json.Unmarshal(bodyBytes, &user)

	return user, err
}
