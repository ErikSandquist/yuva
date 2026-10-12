package helpdesk

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	HelpScoutAuthorize = "https://secure.helpscout.net/authentication/authorizeClientApplication"
	HelpScoutToken     = "https://api.helpscout.net/v2/oauth2/token"
	HelpScoutAPIHost   = "api.helpscout.net"
	ZendeskScope       = "read"
)

func ZendeskHost(subdomain string) string { return subdomain + ".zendesk.com" }

func ZendeskToken(subdomain string) string {
	return "https://" + ZendeskHost(subdomain) + "/oauth/tokens"
}

// App is the OAuth client the team created in its own help desk account.
type App struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// Tokens are what a sign-in or a refresh gives.
type Tokens struct {
	Access  string    `json:"access_token"`
	Refresh string    `json:"refresh_token,omitempty"`
	Expires time.Time `json:"expires_at,omitzero"`
	Scopes  []string  `json:"scopes,omitempty"`
}

// PKCE is a code verifier and its S256 challenge.
func PKCE() (verifier, challenge string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// ZendeskAuthorizeURL asks for the read scope only, with PKCE.
func ZendeskAuthorizeURL(subdomain, clientID, redirect, state, challenge string) string {
	q := url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirect}, "scope": {ZendeskScope},
		"state": {state}, "code_challenge": {challenge}, "code_challenge_method": {"S256"},
	}
	return "https://" + ZendeskHost(subdomain) + "/oauth/authorizations/new?" + q.Encode()
}

// HelpScoutAuthorizeURL: Help Scout takes the redirect URL from the app's settings and knows no
// scopes or PKCE.
func HelpScoutAuthorizeURL(clientID, state string) string {
	return HelpScoutAuthorize + "?" + url.Values{"client_id": {clientID}, "state": {state}}.Encode()
}

// TokenError is the token endpoint refusing a code or a refresh token.
type TokenError struct {
	Status int
	Code   string
}

func (e *TokenError) Error() string {
	return "the token endpoint answered " + http.StatusText(e.Status) + " " + e.Code
}

type tokenAnswer struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    *int64 `json:"expires_in"`
	Error        string `json:"error"`
}

// ZendeskExchange trades a code for tokens; Zendesk takes JSON.
func ZendeskExchange(ctx context.Context, cfg Config, subdomain string, app App, code, redirect, verifier string) (Tokens, error) {
	body, _ := json.Marshal(map[string]string{
		"grant_type": "authorization_code", "code": code, "client_id": app.ClientID, "client_secret": app.ClientSecret,
		"redirect_uri": redirect, "scope": ZendeskScope, "code_verifier": verifier,
	})
	return token(ctx, cfg, ZendeskToken(subdomain), "application/json", body)
}

func ZendeskRefresh(ctx context.Context, cfg Config, subdomain string, app App, refresh string) (Tokens, error) {
	body, _ := json.Marshal(map[string]string{
		"grant_type": "refresh_token", "refresh_token": refresh, "client_id": app.ClientID, "client_secret": app.ClientSecret,
		"scope": ZendeskScope,
	})
	return token(ctx, cfg, ZendeskToken(subdomain), "application/json", body)
}

func HelpScoutExchange(ctx context.Context, cfg Config, app App, code string) (Tokens, error) {
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {app.ClientID}, "client_secret": {app.ClientSecret}}
	return token(ctx, cfg, HelpScoutToken, "application/x-www-form-urlencoded", []byte(form.Encode()))
}

func HelpScoutRefresh(ctx context.Context, cfg Config, app App, refresh string) (Tokens, error) {
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {app.ClientID}, "client_secret": {app.ClientSecret}}
	return token(ctx, cfg, HelpScoutToken, "application/x-www-form-urlencoded", []byte(form.Encode()))
}

func token(ctx context.Context, cfg Config, tokenURL, contentType string, body []byte) (Tokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(body))
	if err != nil {
		return Tokens{}, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	res, err := cfg.tokenClient(tokenURL).Do(req)
	if err != nil {
		return Tokens{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if err != nil {
		return Tokens{}, err
	}
	var a tokenAnswer
	_ = json.Unmarshal(raw, &a)
	if res.StatusCode < 200 || res.StatusCode > 299 || a.AccessToken == "" {
		return Tokens{}, &TokenError{Status: res.StatusCode, Code: a.Error}
	}
	t := Tokens{Access: a.AccessToken, Refresh: a.RefreshToken, Scopes: strings.Fields(a.Scope)}
	if a.ExpiresIn != nil && *a.ExpiresIn > 0 {
		t.Expires = cfg.now().Add(time.Duration(*a.ExpiresIn) * time.Second)
	}
	return t, nil
}
