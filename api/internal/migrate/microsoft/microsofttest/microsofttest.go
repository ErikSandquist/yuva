// Package microsofttest imitates the Microsoft identity platform for tests: the authorize and
// token endpoints of login.microsoftonline.com over TLS (codes bound to PKCE, refresh tokens,
// the errors Microsoft answers), and a token introspection endpoint over plain HTTP that an IMAP
// server such as Dovecot asks whether a token is valid and whose it is.
package microsofttest

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// SignIn is what the next sign-in at the authorize endpoint does: who signs in, or the error
// Microsoft sends back instead.
type SignIn struct {
	Account string
	// Mailbox is the address the token opens over IMAP when it is not Account, as when a
	// Microsoft 365 UPN differs from the mailbox's SMTP address or the account has full access to
	// another mailbox.
	Mailbox string
	// Personal gives an opaque access token that names no account, as Microsoft does for
	// Outlook.com; otherwise the token is a JWT with the account as upn.
	Personal bool
	// Scope is what the token answer says was granted; the IMAP scope when empty.
	Scope string
	// Error and Description are sent back on the redirect instead of a code.
	Error       string
	Description string
}

// Request is one request the TLS server saw.
type Request struct {
	Method string
	Host   string
	Path   string
	Form   url.Values
}

type grant struct {
	sign      SignIn
	client    string
	redirect  string
	challenge string
	tenant    string
}

type token struct {
	account  string
	mailbox  string
	expires  time.Time
	refresh  bool
	scope    string
	client   string
	personal bool
}

type Server struct {
	Pool *x509.CertPool
	// Lifetime is how long access tokens live on the clock New was given.
	Lifetime time.Duration
	// ExchangeError, when set, is what the token endpoint answers to the next code.
	ExchangeError string

	t         testing.TB
	srv       *httptest.Server
	intro     *http.Server
	introAddr string
	now       func() time.Time
	mu        sync.Mutex
	next      SignIn
	apps      map[string]string
	grants    map[string]grant
	tokens    map[string]*token
	requests  []Request
	tenants   []string
	refreshes int
	intros    int
}

// New starts the TLS server for login.microsoftonline.com and, at introspection ("host:port",
// or a free port when empty), the introspection endpoint. now is the clock tokens expire on.
func New(t testing.TB, now func() time.Time, introspection string) *Server {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "login.microsoftonline.com fixture"},
		DNSNames:  []string{"login.microsoftonline.com"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	s := &Server{Pool: pool, Lifetime: time.Hour, t: t, now: now, apps: map[string]string{}, grants: map[string]grant{}, tokens: map[string]*token{}}
	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(s.serve))
	s.srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	s.srv.StartTLS()
	t.Cleanup(s.srv.Close)
	if introspection == "" {
		introspection = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", introspection)
	if err != nil {
		t.Fatal(err)
	}
	s.introAddr = ln.Addr().String()
	s.intro = &http.Server{Handler: http.HandlerFunc(s.introspect), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = s.intro.Serve(ln) }()
	t.Cleanup(func() { _ = s.intro.Close() })
	return s
}

// Register adds an app registration.
func (s *Server) Register(clientID, secret string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apps[clientID] = secret
}

// Next sets what the next sign-in does.
func (s *Server) Next(in SignIn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next = in
}

// IntrospectionURL is where the IMAP server asks about a token.
func (s *Server) IntrospectionURL() string { return "http://" + s.introAddr + "/introspect" }

// Dial connects login.microsoftonline.com to the server.
func (s *Server) Dial(ctx context.Context, _, addr string) (net.Conn, error) {
	host, _, _ := net.SplitHostPort(addr)
	if host != "login.microsoftonline.com" {
		return nil, fmt.Errorf("microsofttest: no route to %s", addr)
	}
	var d net.Dialer
	return d.DialContext(ctx, "tcp", s.srv.Listener.Addr().String())
}

func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Tenants lists the tenant of every authorize request.
func (s *Server) Tenants() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.tenants...)
}

// Refreshes counts refresh-token grants answered with tokens.
func (s *Server) Refreshes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshes
}

// Introspections counts the IMAP server's questions.
func (s *Server) Introspections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.intros
}

// RevokeRefresh makes every refresh token stop working, as when consent is withdrawn.
func (s *Server) RevokeRefresh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, tk := range s.tokens {
		if tk.refresh {
			tk.expires = time.Time{}
		}
	}
}

// Issued lists the access and refresh tokens handed out, so tests can look for them where they
// must never be.
func (s *Server) Issued() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.tokens))
	for v := range s.tokens {
		out = append(out, v)
	}
	return out
}

// Authorize plays the browser at the authorize URL: it answers the redirect back, with a code
// or the error the next sign-in has.
func (s *Server) Authorize(authorizeURL string) (*url.URL, error) {
	u, err := url.Parse(authorizeURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" || u.Host != "login.microsoftonline.com" {
		return nil, fmt.Errorf("not Microsoft's authorize endpoint: %s", u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || parts[1] != "oauth2" || parts[2] != "v2.0" || parts[3] != "authorize" {
		return nil, fmt.Errorf("not the v2.0 authorize path: %s", u.Path)
	}
	q := u.Query()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenants = append(s.tenants, parts[0])
	if _, ok := s.apps[q.Get("client_id")]; !ok {
		return nil, fmt.Errorf("unknown client %q", q.Get("client_id"))
	}
	if q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("state") == "" {
		return nil, fmt.Errorf("not a code request with PKCE: %s", u.RawQuery)
	}
	back, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		return nil, err
	}
	next := s.next
	out := url.Values{"state": {q.Get("state")}}
	if next.Error != "" {
		out.Set("error", next.Error)
		out.Set("error_description", next.Description)
	} else {
		code := random("code")
		s.grants[code] = grant{sign: next, client: q.Get("client_id"), redirect: q.Get("redirect_uri"), challenge: q.Get("code_challenge"), tenant: parts[0]}
		out.Set("code", code)
	}
	back.RawQuery = out.Encode()
	return back, nil
}

func random(prefix string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}

func (s *Server) issue(account, mailbox, scope, client string, personal bool) (string, string) {
	if mailbox == "" {
		mailbox = account
	}
	exp := s.now().Add(s.Lifetime)
	var access string
	if personal {
		access = "EwB" + base64.RawURLEncoding.EncodeToString([]byte(random("opaque")))
	} else {
		head := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"RS256"}`))
		body, _ := json.Marshal(map[string]any{
			"aud": "https://outlook.office.com", "upn": account, "unique_name": account, "scp": "IMAP.AccessAsUser.All",
			"exp": exp.Unix(), "jti": random("jti"),
		})
		access = head + "." + base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString([]byte(random("sig")))
	}
	refresh := random("refresh")
	s.tokens[access] = &token{account: account, mailbox: mailbox, expires: exp, scope: scope, client: client, personal: personal}
	s.tokens[refresh] = &token{account: account, mailbox: mailbox, expires: s.now().Add(90 * 24 * time.Hour), refresh: true, scope: scope, client: client, personal: personal}
	return access, refresh
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	s.mu.Lock()
	s.requests = append(s.requests, Request{Method: r.Method, Host: r.Host, Path: r.URL.Path, Form: r.PostForm})
	s.mu.Unlock()
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if r.Method != http.MethodPost || len(parts) != 4 || parts[1] != "oauth2" || parts[2] != "v2.0" || parts[3] != "token" {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f := r.PostForm
	if secret, ok := s.apps[f.Get("client_id")]; !ok || secret != f.Get("client_secret") {
		tokenError(w, http.StatusUnauthorized, "invalid_client", "AADSTS7000215: Invalid client secret provided.", 7000215)
		return
	}
	switch f.Get("grant_type") {
	case "authorization_code":
		g, ok := s.grants[f.Get("code")]
		delete(s.grants, f.Get("code"))
		sum := sha256.Sum256([]byte(f.Get("code_verifier")))
		switch {
		case !ok || g.client != f.Get("client_id") || g.tenant != parts[0]:
			tokenError(w, http.StatusBadRequest, "invalid_grant", "AADSTS70000: The provided authorization code is invalid or has expired.", 70000)
			return
		case g.redirect != f.Get("redirect_uri"):
			tokenError(w, http.StatusBadRequest, "invalid_grant", "AADSTS50011: The redirect URI specified in the request does not match.", 50011)
			return
		case base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge:
			tokenError(w, http.StatusBadRequest, "invalid_grant", "AADSTS501481: The Code_Verifier does not match the code_challenge supplied in the authorization request.", 501481)
			return
		case s.ExchangeError != "":
			code, desc, _ := strings.Cut(s.ExchangeError, " ")
			s.ExchangeError = ""
			tokenError(w, http.StatusBadRequest, code, desc, 0)
			return
		}
		scope := g.sign.Scope
		if scope == "" {
			scope = "https://outlook.office.com/IMAP.AccessAsUser.All"
		}
		access, refresh := s.issue(g.sign.Account, g.sign.Mailbox, scope, g.client, g.sign.Personal)
		tokenAnswer(w, access, refresh, scope, s.Lifetime)
	case "refresh_token":
		tk, ok := s.tokens[f.Get("refresh_token")]
		if !ok || !tk.refresh || tk.client != f.Get("client_id") || !s.now().Before(tk.expires) {
			tokenError(w, http.StatusBadRequest, "invalid_grant",
				"AADSTS70000: The user or administrator has revoked the grant, or the refresh token has expired.", 70000)
			return
		}
		access, refresh := s.issue(tk.account, tk.mailbox, tk.scope, tk.client, tk.personal)
		s.refreshes++
		tokenAnswer(w, access, refresh, tk.scope, s.Lifetime)
	default:
		tokenError(w, http.StatusBadRequest, "unsupported_grant_type", "AADSTS70003: The app requested an unsupported grant type.", 70003)
	}
}

func tokenAnswer(w http.ResponseWriter, access, refresh, scope string, life time.Duration) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token_type": "Bearer", "scope": scope, "expires_in": int(life / time.Second), "ext_expires_in": int(life / time.Second),
		"access_token": access, "refresh_token": refresh,
	})
}

func tokenError(w http.ResponseWriter, status int, code, desc string, aadsts int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]any{
		"error": code, "error_description": desc + "\r\nTrace ID: 00000000-0000-0000-0000-000000000000\r\nCorrelation ID: 00000000-0000-0000-0000-000000000000\r\nTimestamp: 2026-10-12 10:00:00Z",
		"error_codes": []int{}, "trace_id": "00000000-0000-0000-0000-000000000000",
	}
	if aadsts != 0 {
		body["error_codes"] = []int{aadsts}
	}
	_ = json.NewEncoder(w).Encode(body)
}

// Opens says whether an access token is active and opens mailbox, as Exchange decides when an
// IMAP client signs in to mailbox with it.
func (s *Server) Opens(mailbox, access string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	tk, ok := s.tokens[access]
	return ok && !tk.refresh && s.now().Before(tk.expires) && strings.EqualFold(tk.mailbox, mailbox)
}

// introspect answers whether a token is active and whose mailbox it opens, as RFC 7662 does; the token comes
// as the form's token, a bearer header or the path's last segment, whichever the server sends.
func (s *Server) introspect(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	form, _ := url.ParseQuery(string(body))
	v := form.Get("token")
	if v == "" {
		v = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	if v == "" {
		v = r.URL.Query().Get("token")
	}
	s.mu.Lock()
	s.intros++
	tk, ok := s.tokens[v]
	active := ok && !tk.refresh && s.now().Before(tk.expires)
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if !active {
		_ = json.NewEncoder(w).Encode(map[string]any{"active": false})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "username": tk.mailbox, "scope": tk.scope})
}
