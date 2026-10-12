package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/migrate"
	"github.com/productdevbook/yuva/api/internal/migrate/microsoft"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const microsoftDetectTimeout = 5 * time.Second

// microsoftSignIn is a Microsoft sign-in between its start and Microsoft's redirect back: the
// team's app registration (or that the server's is used), the authority and the PKCE verifier.
type microsoftSignIn struct {
	App       microsoft.App `json:"app"`
	ServerApp bool          `json:"server_app,omitempty"`
	Tenant    string        `json:"tenant"`
	Verifier  string        `json:"verifier"`
}

// microsoftSecret is what a Microsoft sign-in's sealed credentials hold. A refresh uses the app
// the sign-in did: the team's, or the server's as it is configured then.
type microsoftSecret struct {
	App       microsoft.App    `json:"app"`
	ServerApp bool             `json:"server_app,omitempty"`
	Tenant    string           `json:"tenant"`
	Tokens    microsoft.Tokens `json:"tokens"`
	Account   string           `json:"account"`
}

func (sec microsoftSecret) secrets() []string {
	return []string{sec.Tokens.Access, sec.Tokens.Refresh, sec.App.ClientSecret}
}

func microsoftMigration(m store.Migration) bool {
	return m.Source == string(oas.MigrationSourceImap) && m.Preset == migrate.PresetMicrosoft
}

func (s *Server) microsoftServerApp() (microsoft.App, bool) {
	app := microsoft.App{ClientID: s.migrations.MicrosoftClientID, ClientSecret: s.migrations.MicrosoftClientSecret}
	return app, app.ClientID != "" && app.ClientSecret != ""
}

// microsoftAvailable: Sign in with Microsoft works with the server's own app, or with a team's
// when the redirect URL is one Microsoft accepts for a web app (HTTPS, or http on localhost).
func (s *Server) microsoftAvailable() bool {
	if _, ok := s.microsoftServerApp(); ok {
		return true
	}
	u, err := url.Parse(s.signInRedirectURL())
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		h := u.Hostname()
		return h == "localhost" || h == "127.0.0.1" || h == "::1"
	}
	return false
}

func (s *Server) presetAvailable(p migrate.Preset) bool {
	if p.ID == migrate.PresetMicrosoft {
		return s.microsoftAvailable()
	}
	return p.Available
}

func (s *Server) microsoftConfig() microsoft.Config {
	cfg := microsoft.Config{RootCAs: s.migrations.RootCAs, Now: s.now, Timeout: signInTimeout, Dial: s.checkedDial}
	if s.migrations.MicrosoftDial != nil {
		cfg.Dial = s.migrations.MicrosoftDial
	}
	return cfg
}

// sourceDial is how an IMAP connection to host reaches it: after the address checks, or for
// Microsoft's server through the test dialer when one is set.
func (s *Server) sourceDial(host string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	if s.migrations.MicrosoftDial != nil && strings.EqualFold(host, microsoft.IMAPHost) {
		return s.migrations.MicrosoftDial
	}
	return nil
}

// microsoftTenant picks the authority: the team's tenant for its single-tenant app, the server
// app's tenant when it is not multi-tenant, else by the address: consumers, organizations or
// common.
func (s *Server) microsoftTenant(ctx context.Context, address, team string, serverApp bool) string {
	if team != "" {
		return team
	}
	if t := s.migrations.MicrosoftTenant; serverApp && t != "" && !strings.EqualFold(t, microsoft.AuthorityCommon) {
		return t
	}
	personal, work := migrate.MicrosoftAccount(ctx, s.migrations.Resolver, address, microsoftDetectTimeout)
	return microsoft.Authority(personal, work)
}

func (s *Server) startMicrosoftSignIn(ctx context.Context, p principal, m store.Migration, b *oas.StartMigrationSignInJSONRequestBody) (oas.StartMigrationSignInResponseObject, error) {
	if !s.microsoftAvailable() {
		return nil, errProviderNotConfigured
	}
	clientID, secret, tenant := strings.TrimSpace(b.ClientId), "", ""
	if b.ClientSecret != nil {
		secret = strings.TrimSpace(*b.ClientSecret)
	}
	if b.Tenant != nil {
		tenant = strings.TrimSpace(*b.Tenant)
	}
	serverApp, hasServerApp := s.microsoftServerApp()
	team := clientID != "" || secret != ""
	switch {
	case team && (len(clientID) < 1 || len(clientID) > 200):
		return nil, errValidation("client_id must be the app registration's Application (client) ID, 1 to 200 characters")
	case team && (len(secret) < 1 || len(secret) > 500):
		return nil, errValidation("client_secret must be 1 to 500 characters")
	case !team && !hasServerApp:
		return nil, errValidation("this server has no Microsoft app of its own: give client_id and client_secret of your app registration")
	case tenant != "" && !team:
		return nil, errValidation("tenant goes with your own app registration's client_id and client_secret")
	case tenant != "" && !microsoft.ValidTenant(tenant):
		return nil, errValidation("tenant must be the Directory (tenant) ID or a verified domain")
	}
	switch m.State {
	case migrationDraft, migrationConnected, migrationNeedsAttention:
	default:
		return nil, errMigrationState("Microsoft signs in while the migration is a draft, connected or needs attention")
	}
	tenant = s.microsoftTenant(ctx, m.SourceAddress, tenant, !team)
	app := microsoft.App{ClientID: clientID, ClientSecret: secret}
	if !team {
		app = serverApp
	}
	var out oas.StartMigrationSignIn200JSONResponse
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		cur, err := visibleMigration(ctx, q, p, m.ID, true)
		if err != nil {
			return err
		}
		if !microsoftMigration(cur) {
			return errNotHelpdesk
		}
		switch cur.State {
		case migrationDraft, migrationConnected, migrationNeedsAttention:
		default:
			return errMigrationState("Microsoft signs in while the migration is a draft, connected or needs attention")
		}
		state := signInState(cur.WorkspaceID, cur.ID)
		verifier, challenge := microsoft.PKCE()
		sec := microsoftSignIn{ServerApp: !team, Tenant: tenant, Verifier: verifier}
		if team {
			sec.App = app
		}
		var keyID *uuid.UUID
		if p.isKey() {
			keyID = &p.keyID
		}
		now := s.now()
		if err := q.SetMigrationSignIn(ctx, store.SetMigrationSignInParams{
			WorkspaceID: cur.WorkspaceID, MigrationID: cur.ID, StateHash: stateHash(state),
			Sealed: s.secrets.Seal(mustJSON(sec), migrationSecretContext(cur.WorkspaceID, cur.ID)), Host: microsoft.LoginHost,
			MemberID: p.memberRef(), ApiKeyID: keyID, ExpiresAt: now.Add(signInTTL), Now: now,
		}); err != nil {
			return err
		}
		appKind := "team"
		if !team {
			appKind = "server"
		}
		if _, err := s.migrationAudit(ctx, q, p, cur.ID, "sign_in_started", map[string]any{"host": microsoft.LoginHost, "tenant": tenant, "app": appKind}); err != nil {
			return err
		}
		out = oas.StartMigrationSignIn200JSONResponse{
			AuthorizeUrl: microsoft.AuthorizeURL(tenant, app.ClientID, s.signInRedirectURL(), state, challenge, cur.SourceAddress),
			RedirectUrl:  s.signInRedirectURL(), ExpiresAt: now.Add(signInTTL),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func microsoftSignInState(state string) bool {
	return state == migrationDraft || state == migrationConnected || state == migrationNeedsAttention
}

// finishMicrosoftSignIn takes Microsoft's redirect back: it exchanges the code, refuses other
// scopes, signs in to the mailbox over IMAP with the token, and only then stores the tokens. The
// token's account may differ from the mailbox (a Microsoft 365 UPN is often not the SMTP address,
// and a delegate with full access opens another's mailbox): Exchange accepting the token for the
// mailbox decides, and its refusal of a token that names another account is account_mismatch.
func (s *Server) finishMicrosoftSignIn(ctx context.Context, row store.MigrationSignIn, m store.Migration, query url.Values) oas.MigrationSignInResult {
	ws, mid := m.WorkspaceID, m.ID
	if !microsoftSignInState(m.State) {
		return oas.SignInExpired
	}
	plain, err := s.secrets.Open(row.Sealed, migrationSecretContext(ws, mid))
	var sec microsoftSignIn
	if err != nil || json.Unmarshal(plain, &sec) != nil {
		return s.signInFailed(ctx, ws, mid, oas.SignInExpired)
	}
	app := sec.App
	if sec.ServerApp {
		var ok bool
		if app, ok = s.microsoftServerApp(); !ok {
			return s.signInFailed(ctx, ws, mid, oas.SignInTokenFailed)
		}
	}
	code := query.Get("code")
	if e := query.Get("error"); e != "" || code == "" {
		if e == "" {
			e = "access_denied"
		}
		return s.microsoftRefused(ctx, m, app, microsoft.NewError(0, e, query.Get("error_description"), nil))
	}
	cctx, cancel := context.WithTimeout(ctx, signInTimeout)
	defer cancel()
	tokens, err := microsoft.Exchange(cctx, s.microsoftConfig(), sec.Tenant, app, code, s.signInRedirectURL(), sec.Verifier)
	if err != nil {
		var me *microsoft.Error
		if errors.As(err, &me) {
			return s.microsoftRefused(ctx, m, app, me)
		}
		s.log.WarnContext(ctx, "migration sign-in token", slog.String("migration_id", mid.String()), slog.String("error", migrate.Scrub(err.Error(), app.ClientSecret, code)))
		return s.signInFailed(ctx, ws, mid, oas.SignInSourceUnreachable)
	}
	secret := microsoftSecret{App: sec.App, ServerApp: sec.ServerApp, Tenant: sec.Tenant, Tokens: tokens, Account: m.SourceAddress}
	if microsoft.CheckScopes(tokens.Scopes) != nil || tokens.Refresh == "" {
		return s.signInFailed(ctx, ws, mid, oas.SignInScopeTooBroad)
	}
	accounts := microsoft.Accounts(tokens.Access)
	named := slices.Contains(accounts, strings.ToLower(m.SourceAddress))
	account := ""
	switch {
	case named:
		account = strings.ToLower(m.SourceAddress)
	case len(accounts) > 0:
		account = accounts[0]
	}
	preset, _ := migrate.LookupPreset(migrate.PresetMicrosoft)
	res := s.microsoftTester().Run(ctx, migrate.Target{
		Preset: preset, Server: preset.Servers[0], Username: m.SourceAddress, Token: tokens.Access,
	})
	if !res.OK {
		failed := res.Failed()
		s.log.InfoContext(ctx, "migration sign-in mailbox", slog.String("migration_id", mid.String()), slog.String("step", failed.Step),
			slog.String("code", failed.Code), slog.String("detail", migrate.Scrub(failed.Detail, secret.secrets()...)))
		switch {
		case failed.Code == migrate.CodeXOAUTH2Failed && named:
			return s.signInFailed(ctx, ws, mid, oas.SignInXoauth2Failed)
		case failed.Code == migrate.CodeXOAUTH2Failed:
			return s.signInFailed(ctx, ws, mid, oas.SignInAccountMismatch)
		}
		return s.signInFailed(ctx, ws, mid, oas.SignInSourceUnreachable)
	}
	sv := preset.Servers[0]
	result := oas.SignInConnected
	err = s.inTx(ctx, ws, func(q *store.Queries, events *eventBatch) error {
		cur, err := q.LockMigration(ctx, store.LockMigrationParams{WorkspaceID: ws, ID: mid})
		if err != nil {
			return err
		}
		if !microsoftMigration(cur) || !microsoftSignInState(cur.State) {
			result = oas.SignInExpired
			return nil
		}
		if err := q.SetMigrationCredentials(ctx, store.SetMigrationCredentialsParams{
			WorkspaceID: ws, MigrationID: mid, Kind: string(oas.CredentialsOauth), Sealed: s.secrets.Seal(mustJSON(secret), migrationSecretContext(ws, mid)),
			Host: sv.Host, Port: int32(sv.Port), Tls: sv.TLS, Scopes: tokens.Scopes, ExpiresAt: expiresAtOf(tokens.Expires), Now: s.now(),
		}); err != nil {
			return err
		}
		if cur.State != migrationNeedsAttention {
			listed := make([]listedFolder, len(res.List))
			for i, f := range res.List {
				listed[i] = listedFolder{Name: f.Name, Delim: f.Delim, Kind: f.Kind, Messages: f.Messages}
			}
			if err := s.recordFolders(ctx, q, cur, listed); err != nil {
				return err
			}
		}
		doc := readSourceState(cur.SourceState)
		doc.SignInResult, doc.AdminConsentURL = string(oas.SignInConnected), ""
		if err := q.SetMigrationSourceState(ctx, store.SetMigrationSourceStateParams{WorkspaceID: ws, ID: mid, SourceState: mustJSON(doc)}); err != nil {
			return err
		}
		if _, err := q.AddMigrationAudit(ctx, store.AddMigrationAuditParams{
			WorkspaceID: ws, MigrationID: mid, Action: "signed_in", MemberID: row.MemberID, ApiKeyID: row.ApiKeyID,
			Detail: mustJSON(signedInDetail(sv.Host, tokens.Scopes, sec.Tenant, account)), Now: s.now(),
		}); err != nil {
			return err
		}
		m := cur
		if cur.State == migrationDraft {
			if m, err = q.SetMigrationState(ctx, store.SetMigrationStateParams{WorkspaceID: ws, ID: mid, State: migrationConnected, Now: s.now()}); err != nil {
				return err
			}
		}
		m.SourceState = mustJSON(doc)
		return s.migrationEvent(ctx, q, events, m, &cur.State)
	})
	if err != nil {
		s.log.ErrorContext(ctx, "migration sign-in", slog.Any("error", err))
		return oas.SignInSourceUnreachable
	}
	if result != oas.SignInConnected {
		return s.signInFailed(ctx, ws, mid, result)
	}
	return result
}

func signedInDetail(host string, scopes []string, tenant, account string) map[string]any {
	d := map[string]any{"host": host, "scopes": scopes, "tenant": tenant}
	if account != "" {
		d["account"] = account
	}
	return d
}

// microsoftRefused records Microsoft's refusal of a sign-in, with the admin consent page of the
// mailbox's organization when only an admin can consent.
func (s *Server) microsoftRefused(ctx context.Context, m store.Migration, app microsoft.App, e *microsoft.Error) oas.MigrationSignInResult {
	result := oas.MigrationSignInResult(e.Result())
	admin := ""
	if result == oas.SignInAdminConsentRequired {
		_, domain, _ := strings.Cut(m.SourceAddress, "@")
		admin = microsoft.AdminConsentURL(domain, app.ClientID, s.signInRedirectURL())
	}
	s.log.InfoContext(ctx, "migration sign-in refused", slog.String("migration_id", m.ID.String()), slog.String("result", string(result)),
		slog.String("code", e.Code), slog.Any("aadsts", e.AADSTS))
	return s.signInEnded(ctx, m.WorkspaceID, m.ID, result, admin)
}

func expiresAtOf(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (s *Server) microsoftTester() migrate.Tester {
	return migrate.Tester{
		Resolver: s.migrations.Resolver, AllowPrivate: s.migrations.AllowPrivate, RootCAs: s.migrations.RootCAs, Timeout: s.migrations.TestTimeout,
		Dial: s.sourceDial(microsoft.IMAPHost),
	}
}

// microsoftCredentials opens a Microsoft sign-in's credentials and, when the access token expires
// within five minutes, refreshes it and stores the new tokens sealed before they are used.
func (s *Server) microsoftCredentials(ctx context.Context, m store.Migration, cred store.MigrationCredential) (microsoftSecret, error) {
	var sec microsoftSecret
	plain, err := s.secrets.Open(cred.Sealed, migrationSecretContext(m.WorkspaceID, m.ID))
	if err != nil {
		return sec, err
	}
	if err := json.Unmarshal(plain, &sec); err != nil {
		return sec, err
	}
	if sec.Tokens.Expires.IsZero() || s.now().Add(tokenRefreshAhead).Before(sec.Tokens.Expires) {
		return sec, nil
	}
	app := sec.App
	if sec.ServerApp {
		var ok bool
		if app, ok = s.microsoftServerApp(); !ok {
			return sec, microsoft.NewError(0, "unauthorized_client", "the server's Microsoft app is no longer configured", nil)
		}
	}
	if sec.Tokens.Refresh == "" {
		return sec, microsoft.NewError(0, "invalid_grant", "no refresh token is stored", nil)
	}
	cctx, cancel := context.WithTimeout(ctx, signInTimeout)
	defer cancel()
	t, err := microsoft.Refresh(cctx, s.microsoftConfig(), sec.Tenant, app, sec.Tokens.Refresh)
	if err != nil {
		return sec, err
	}
	if len(t.Scopes) == 0 {
		t.Scopes = sec.Tokens.Scopes
	}
	if err := microsoft.CheckScopes(t.Scopes); err != nil {
		return sec, err
	}
	sec.Tokens = t
	if _, err := s.st.ResealMigrationCredentials(ctx, store.ResealMigrationCredentialsParams{
		WorkspaceID: m.WorkspaceID, MigrationID: m.ID, Sealed: s.secrets.Seal(mustJSON(sec), migrationSecretContext(m.WorkspaceID, m.ID)),
		Scopes: t.Scopes, ExpiresAt: expiresAtOf(t.Expires), Previous: cred.Sealed,
	}); err != nil {
		return sec, err
	}
	return sec, nil
}

// microsoftTestFailure is the connection test's answer when the token could not be refreshed.
func microsoftTestFailure(err error, sec microsoftSecret) migrate.TestResult {
	code := migrate.CodeUnexpectedAnswer
	var me *microsoft.Error
	var se *microsoft.ScopeError
	switch {
	case errors.As(err, &me):
		code = migrate.CodeTokenRefreshFailed
	case errors.As(err, &se):
		code = migrate.CodeScopeTooBroad
	}
	return migrate.TestResult{Steps: []migrate.TestStep{{Step: migrate.StepSignIn, Code: code, Detail: migrate.Scrub(err.Error(), sec.secrets()...)}}}
}
