package internal

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"main/internal/idp"

	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

func GetClient(r *http.Request) string {
	client := r.URL.Query()["client"]
	if len(client) != 1 || !strings.Contains(cfg.TrustedClients, client[0]+",") {
		Logger.Sugar().Debugf("bad client: %v", client)
		return ""
	}
	return client[0]
}

func CheckCookie(r *http.Request) (string, error) {

	//auth cookie
	// return checkAuthCookie(r)

	//jwt cookie
	claims, err := checkJwtCookie(r)
	if err != nil {
		if cfg.AllowAuthCookie { // (dev env only) or if you have a good authCookie ?
			Logger.Sugar().Debugf("bad jwtCookie(err: %v) + AllowAuthCookie == falling back to authCookie", err)
			Logger.Sugar().Debugf("dump r, %v", r)
			return checkAuthCookie(r)
		} else {
			return "", err
		}
	}
	Logger.Sugar().Debugf("good jwt cookie, jwt.MapClaims: %v", claims)
	return claims.(jwt.MapClaims)["fxa_email"].(string), nil
}

func checkAuthCookie(r *http.Request) (string, error) {
	// Get auth cookie
	c, err := r.Cookie(cfg.CookieName)
	if err != nil {
		Logger.Sugar().Debug("missing cookie: " + cfg.CookieName)
		return "", errors.New("missing cookie")
	}
	// Validate cookie
	email, err := ValidateCookie(r, c)
	if err != nil {
		if err.Error() != "Cookie has expired" {
			Logger.Sugar().Warn("Bad cookie, err: " + err.Error())
		}
		return "", err
	}

	return email, nil
}

func checkJwtCookie(r *http.Request) (jwt.Claims, error) {
	// Get auth cookie
	c, err := r.Cookie(cfg.JwtCookieName)
	if err != nil {
		Logger.Sugar().Debug("missing jwtCookie: " + cfg.JwtCookieName)
		return nil, errors.New("missing jwtCookie")
	}
	// Validate cookie

	// Logger.Sugar().Debugf("cfg.PermsKey.Public(): %v", cfg.PermsKey.Public())
	// Logger.Sugar().Debugf("cfg.PermsKey.PublicKey: %v", cfg.PermsKey.PublicKey)

	token, err := jwt.Parse(c.Value, func(token *jwt.Token) (interface{}, error) {
		// since we only use the one private key to sign the tokens,
		// we also only use its public counter part to verify
		return cfg.PermsKey_pub, nil
	})
	if err != nil {
		return nil, err
	}
	Logger.Sugar().Debugf("token: %v", token)
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	// good token
	return token.Claims, nil

	// Logger.Sugar().Debugf("token: %v", token)
	// Logger.Sugar().Debugf("token.Claims: %v", token.Claims)

	// branch out into the possible error from signing

	// switch err.(type) {

	// case nil: // no error

	// 	if !token.Valid { // but may still be invalid
	// 		Logger.Debug("invalid token: " + err.Error())
	// 		return "", errors.New("invalid token")
	// 	}
	// 	// good token
	// 	Logger.Sugar().Debugf("good token -- token.Raw: %v", token.Raw)
	// 	claims := token.Claims.(jwt.MapClaims)
	// 	return claims["fxa_email"].(string), nil

	// case *jwt.ValidationError: // something was wrong during the validation
	// 	vErr := err.(*jwt.ValidationError)
	// 	switch vErr.Errors {
	// 	case jwt.ValidationErrorExpired:
	// 		Logger.Debug("token expired: " + err.Error())
	// 		return "", err
	// 	default:
	// 		Logger.Debug("jwt.ValidationError -- unexpected: " + err.Error())
	// 		return "", err
	// 	}

	// default: // something else went wrong
	// 	Logger.Debug("unexpected error: " + err.Error())
	// 	return "", err
	// }
}

// Request Validation

// ValidateCookie verifies that a cookie matches the expected format of:
// Cookie = hash(secret, cookie domain, email, expires)|expires|email
func ValidateCookie(r *http.Request, c *http.Cookie) (string, error) {
	parts := strings.Split(c.Value, "|")

	if len(parts) != 3 {
		return "", errors.New("invalid cookie format")
	}

	mac, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errors.New("unable to decode cookie mac")
	}

	expectedSignature := cookieSignature(r, parts[2], parts[1])
	expected, err := base64.URLEncoding.DecodeString(expectedSignature)
	if err != nil {
		return "", errors.New("unable to generate mac")
	}

	// Valid token?
	if !hmac.Equal(mac, expected) {
		return "", errors.New("invalid cookie mac")
	}

	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", errors.New("unable to parse cookie expiry")
	}

	// Has it expired?
	if time.Unix(expires, 0).Before(time.Now()) {
		return "", errors.New("cookie has expired")
	}

	// Looks valid
	return parts[2], nil
}

// ValidateEmail checks if the given email address matches either a whitelisted
// email address, as defined by the "whitelist" config parameter. Or is part of
// a permitted domain, as defined by the "domains" config parameter
func ValidateEmail(email, ruleName string) bool {
	// Use global config by default
	whitelist := cfg.Whitelist
	domains := cfg.Domains

	if rule, ok := cfg.Rules[ruleName]; ok {
		// Override with rule config if found
		if len(rule.Whitelist) > 0 || len(rule.Domains) > 0 {
			whitelist = rule.Whitelist
			domains = rule.Domains
		}
	}

	// Do we have any validation to perform?
	if len(whitelist) == 0 && len(domains) == 0 {
		return true
	}

	// Email whitelist validation
	if len(whitelist) > 0 {
		if ValidateWhitelist(email, whitelist) {
			return true
		}

		// If we're not matching *either*, stop here
		if !cfg.MatchWhitelistOrDomain {
			return false
		}
	}

	// Domain validation
	if len(domains) > 0 && ValidateDomains(email, domains) {
		return true
	}

	return false
}

// ValidateWhitelist checks if the email is in whitelist
func ValidateWhitelist(email string, whitelist CommaSeparatedList) bool {
	for _, whitelist := range whitelist {
		if email == whitelist {
			return true
		}
	}
	return false
}

// ValidateDomains checks if the email matches a whitelisted domain
func ValidateDomains(email string, domains CommaSeparatedList) bool {
	parts := strings.Split(email, "@")
	if len(parts) < 2 {
		return false
	}
	for _, domain := range domains {
		if domain == parts[1] {
			return true
		}
	}
	return false
}

// Should we use auth host + what it is
func useAuthDomain(r *http.Request) (bool, string) {
	if cfg.AuthHost == "" {
		return false, ""
	}

	// if r.Header.Get("x-turkeyauth-proxied") != "" {
	// 	Logger.Debug("force cfg.AuthHost because -H x-turkeyauth-proxied == " + r.Header.Get("x-turkeyauth-proxied"))
	// 	return true, cfg.AuthHost
	// }

	// Does the request match a given cookie domain?
	reqMatch, reqHost := matchCookieDomains(r.Host)

	// Do any of the auth hosts match a cookie domain?
	authMatch, authHost := matchCookieDomains(cfg.AuthHost)

	use := reqMatch && authMatch && reqHost == authHost
	Logger.Sugar().Debugf("reqMatch: %v,reqHost: %v,authMatch: %v,authHost: %v, use: %v",
		reqMatch, reqHost, authMatch, authHost, use)
	// We need both to match the same domain
	return use, reqHost
}

// Cookie methods

func MakeAuthCookie(r *http.Request, email string) *http.Cookie {
	expires := cookieExpiry()
	mac := cookieSignature(r, email, fmt.Sprintf("%d", expires.Unix()))
	value := fmt.Sprintf("%s|%d|%s", mac, expires.Unix(), email)

	return &http.Cookie{
		Name:     cfg.CookieName,
		Value:    value,
		Path:     "/",
		Domain:   cookieDomain(r),
		HttpOnly: true,
		Secure:   !cfg.InsecureCookie,
		Expires:  expires,
	}
}

func MakeJwtCookie(r *http.Request, user idp.User) (*http.Cookie, error) {
	expires := cookieExpiry()
	// Create a new token object, specifying signing method and the claims
	// you would like it to contain.
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": cfg.TurkeyDomain,
		"sub": user.Sub,
		// "aud":"whomever?",
		"exp": expires.Unix(),
		// "nbf": time.Now().UTC(),
		"iat":             time.Now().Add(-1 * time.Minute).Unix(),
		"fxa_pic":         user.Avatar,
		"fxa_2fa":         user.TwoFA,
		"fxa_email":       user.Email,
		"fxa_displayName": user.DisplayName,
	})
	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString(cfg.PermsKey)
	if err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name:     cfg.JwtCookieName,
		Value:    tokenString,
		Path:     "/",
		Domain:   cookieDomain(r),
		HttpOnly: true,
		Secure:   !cfg.InsecureCookie,
		Expires:  expires,
	}, nil
}

// ClearCookie clears the auth cookie
func ClearCookie(r *http.Request, cookieName string) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Domain:   cookieDomain(r),
		HttpOnly: true,
		Secure:   !cfg.InsecureCookie,
		Expires:  time.Unix(0, 0),
		// Expires:  time.Now().Local().Add(time.Hour * -1),
	}
}

func buildCSRFCookieName(nonce string) string {
	Logger.Debug("nonce: " + nonce)
	return cfg.CSRFCookieName + "_" + nonce[:6]
}

// MakeCSRFCookie makes a csrf cookie (used during login only)
//
// Note, CSRF cookies live shorter than auth cookies, a fixed 1h.
// That's because some CSRF cookies may belong to auth flows that don't complete
// and thus may not get cleared by ClearCookie.
func MakeCSRFCookie(r *http.Request, nonce string) *http.Cookie {
	return &http.Cookie{
		Name:     buildCSRFCookieName(nonce),
		Value:    nonce,
		Path:     "/",
		Domain:   csrfCookieDomain(r),
		HttpOnly: true,
		Secure:   !cfg.InsecureCookie,
		Expires:  time.Now().Local().Add(time.Minute * 9),
	}
}

// ClearCSRFCookie makes an expired csrf cookie to clear csrf cookie
func ClearCSRFCookie(r *http.Request, c *http.Cookie) *http.Cookie {
	return &http.Cookie{
		Name:     c.Name,
		Value:    "",
		Path:     "/",
		Domain:   csrfCookieDomain(r),
		HttpOnly: true,
		Secure:   !cfg.InsecureCookie,
		Expires:  time.Unix(0, 0),
		// Expires:  time.Now().Local().Add(time.Hour * -1),
	}
}

// FindCSRFCookie extracts the CSRF cookie from the request based on state.
func FindCSRFCookie(r *http.Request, state string) (c *http.Cookie, err error) {
	// Check for CSRF cookie
	return r.Cookie(buildCSRFCookieName(state))
}

// ValidateCSRFCookie validates the csrf cookie against state
func ValidateCSRFCookie(c *http.Cookie, state string) (valid bool, provider string, redirect string, err error) {
	if len(c.Value) != 32 {
		return false, "", "", errors.New("invalid CSRF cookie value")
	}
	Logger.Sugar().Debugf("c.Value: %v, state: %v", c.Value, state)
	// Check nonce match
	if c.Value != state[:32] {
		return false, "", "", errors.New("CSRF cookie does not match state")
	}

	// Extract provider
	params := state[33:]
	split := strings.Index(params, ":")
	if split == -1 {
		return false, "", "", errors.New("invalid CSRF state format")
	}

	// Valid, return provider and redirect
	return true, params[:split], params[split+1:], nil
}

// MakeState generates a state value
func MakeState(r *http.Request, p idp.Provider, nonce string) string {
	return fmt.Sprintf("%s:%s:%s", nonce, p.Name(), returnUrl(r))
}

// Return url
func returnUrl(r *http.Request) string {
	client := fmt.Sprintf("%s%s", redirectBase(r), r.URL.Path)
	if len(r.URL.Query()["client"]) == 1 {
		client = r.URL.Query()["client"][0]
	}
	Logger.Debug("returnUrl: " + client)
	return client
}

// Utility methods

// Get the redirect base
func redirectBase(r *http.Request) string {
	return fmt.Sprintf("%s://%s", r.Header.Get("X-Forwarded-Proto"), r.Host)
}

// Get oauth redirect uri
// func redirectUri(r *http.Request) string {
// 	if use, _ := useAuthDomain(r); use {
// 		p := r.Header.Get("X-Forwarded-Proto")
// 		return fmt.Sprintf("%s://%s%s", p, cfg.AuthHost, cfg.Path)
// 	}

// 	return fmt.Sprintf("%s%s", redirectBase(r), cfg.Path)
// }

// ValidateState checks whether the state is of right length.
func ValidateState(state string) error {
	if len(state) < 34 {
		return errors.New("invalid CSRF state value")
	}
	return nil
}

// Nonce generates a random nonce
func Nonce() (string, error) {
	nonce := make([]byte, 16)
	_, err := rand.Read(nonce)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", nonce), nil
}

// Cookie domain
func cookieDomain(r *http.Request) string {
	// Check if any of the given cookie domains matches
	_, domain := matchCookieDomains(r.Host)

	Logger.Sugar().Debugf("cookieDomain (%v)---dump r, %v", domain, r)

	return domain
}

// Cookie domain
func csrfCookieDomain(r *http.Request) string {
	var host string
	if use, domain := useAuthDomain(r); use {
		host = domain
	} else {
		host = r.Host
	}
	// Remove port
	p := strings.Split(host, ":")

	Logger.Sugar().Debugf("csrfCookieDomain (%v)---dump r, %v", p[0], r)

	return p[0]
}

// Return matching cookie domain if exists
func matchCookieDomains(domain string) (bool, string) {
	// Remove port
	p := strings.Split(domain, ":")

	for _, d := range cfg.CookieDomains {
		if d.Match(p[0]) {
			if d.Domain == cfg.Domain {
				Logger.Sugar().Warnf("roodDomain (%v) matched (against: %v), cross-domain-leak possible", d.Domain, domain)
			}
			return true, d.Domain
		}
	}
	Logger.Sugar().Warnf("no match, falling back to: %v", p[0])
	return false, p[0]
}

// Match checks if the given host matches this CookieDomain
func (c *CookieDomain) Match(host string) bool {
	Logger.Sugar().Debugf("matching: %v for %v", host, c)

	// Exact domain match?
	if host == c.Domain {
		return true
	}
	// Subdomain match?
	if len(host) >= c.SubDomainLen && host[len(host)-c.SubDomainLen:] == c.SubDomain {
		return true
	}
	return false
}

// Create cookie hmac
func cookieSignature(r *http.Request, email, expires string) string {

	hash := hmac.New(sha256.New, cfg.Secret)
	cookieDomain := cookieDomain(r)
	// logger.Debug("### cookieSignature ### cookieDomain: " + cookieDomain)

	hash.Write([]byte(cookieDomain))
	hash.Write([]byte(email))
	hash.Write([]byte(expires))
	return base64.URLEncoding.EncodeToString(hash.Sum(nil))
}

// Get cookie expiry
func cookieExpiry() time.Time {
	return time.Now().Local().Add(cfg.Lifetime)
}

// CookieDomain holds cookie domain info
type CookieDomain struct {
	Domain       string
	DomainLen    int
	SubDomain    string
	SubDomainLen int
}

// NewCookieDomain creates a new CookieDomain from the given domain string
func NewCookieDomain(domain string) *CookieDomain {
	return &CookieDomain{
		Domain:       domain,
		DomainLen:    len(domain),
		SubDomain:    fmt.Sprintf(".%s", domain),
		SubDomainLen: len(domain) + 1,
	}
}

// UnmarshalFlag converts a string to a CookieDomain
func (c *CookieDomain) UnmarshalFlag(value string) error {
	*c = *NewCookieDomain(value)
	return nil
}

// MarshalFlag converts a CookieDomain to a string
func (c *CookieDomain) MarshalFlag() (string, error) {
	return c.Domain, nil
}

// CookieDomains provides legacy sypport for comma separated list of cookie domains
type CookieDomains []CookieDomain

// UnmarshalFlag converts a comma separated list of cookie domains to an array
// of CookieDomains
func (c *CookieDomains) UnmarshalFlag(value string) error {
	if len(value) > 0 {
		for _, d := range strings.Split(value, ",") {
			cookieDomain := NewCookieDomain(d)
			*c = append(*c, *cookieDomain)
		}
	}
	return nil
}

// MarshalFlag converts an array of CookieDomain to a comma seperated list
func (c *CookieDomains) MarshalFlag() (string, error) {
	var domains []string
	for _, d := range *c {
		domains = append(domains, d.Domain)
	}
	return strings.Join(domains, ","), nil
}
