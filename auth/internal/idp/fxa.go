package idp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

// Fxa provider
type Fxa struct {
	ClientID     string `long:"client-id" env:"CLIENT_ID" description:"Client ID"`
	ClientSecret string `long:"client-secret" env:"CLIENT_SECRET" description:"Client Secret" json:"-"`
	Scope        string
	entrypoint   string
	Prompt       string `long:"prompt" env:"PROMPT" default:"select_account" description:"Space separated list of OpenID prompt options"`

	LoginURL        *url.URL
	TokenURL        *url.URL
	UserURL         *url.URL
	SubscriptionURL *url.URL

	turkey_prod_id string
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

	// //default to prod fxa
	fxaLoginHost := "accounts.firefox.com"
	fxaTokenHost := "api.accounts.firefox.com"
	fxaUserHost := "profile.accounts.firefox.com"
	fxaSubHost := "api.accounts.firefox.com"
	f.turkey_prod_id = "prod_Mo4tS8uH9y3Mj5"

	if os.Getenv("ENV") == "dev" {
		fxaLoginHost = "accounts.stage.mozaws.net"
		fxaTokenHost = "oauth.stage.mozaws.net"
		fxaUserHost = "profile.stage.mozaws.net"
		fxaSubHost = "api-accounts.stage.mozaws.net"
		f.turkey_prod_id = "prod_KPReWHqwGqZBzc"
	}

	// Set static values
	f.Scope = "profile openid https://identity.mozilla.com/account/subscriptions"
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
	f.SubscriptionURL = &url.URL{
		Scheme: "https",
		Host:   fxaSubHost,
		Path:   "/v1/oauth/mozilla-subscriptions/customer/billing-and-subscriptions",
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
	// fmt.Println("GetUser.res.StatusCode = ", res.StatusCode)

	// err = json.NewDecoder(res.Body).Decode(&user)
	bodyBytes, _ := ioutil.ReadAll(res.Body)
	// fmt.Println("GetUser -- bodyBytes -- " + string(bodyBytes))
	err = json.Unmarshal(bodyBytes, &user)

	return user, err
}

func (f *Fxa) GetSubscriptions(token string, user *User) error {

	client := &http.Client{}

	req, err := http.NewRequest("GET", f.SubscriptionURL.String(), nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	fmt.Println("GetSubscriptions.res.StatusCode = ", res.StatusCode)
	if res.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(res.Body)
		return errors.New(string(bodyBytes))
	}

	bodyBytes, _ := ioutil.ReadAll(res.Body)
	fmt.Println("GetSubscriptions -- bodyBytes -- " + string(bodyBytes))

	err = loadSubInfo(bodyBytes, f.turkey_prod_id, user)

	return err
}

func loadSubInfo(subRespJson []byte, turkey_prod_id string, user *User) error {

	subplatMap := make(map[string]interface{})
	err := json.Unmarshal(subRespJson, &subplatMap)
	if err != nil {
		return err
	}
	subs := subplatMap["subscriptions"]
	for _, sub := range subs.([]interface{}) {
		map_sub := sub.(map[string]interface{})
		product_id := map_sub["product_id"].(string)
		if product_id == turkey_prod_id {
			user.Plan_id = map_sub["plan_id"].(string)
			user.Cancel_at_period_end = map_sub["cancel_at_period_end"].(bool)
			user.Current_period_end = map_sub["current_period_end"].(float64)
			return nil
		}
	}

	return errors.New("no subscription found for project_id: " + turkey_prod_id)
}
