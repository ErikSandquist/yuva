package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/migrate"
	"github.com/productdevbook/yuva/api/internal/migrate/helpdesk"
	"github.com/productdevbook/yuva/api/internal/migrate/microsoft"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	signInCallbackPath = "/migrations/sign-in/callback"
	signInTTL          = 10 * time.Minute
	signInTimeout      = 20 * time.Second
	tokenRefreshAhead  = 5 * time.Minute
	helpdeskPort       = 443
)

var (
	subdomainRe           = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	errNotHelpdesk        = errMigrationState("only a Zendesk, Help Scout or Microsoft migration signs in")
	errHelpdeskNoPassword = errMigrationState("a help desk connects with a sign-in (POST .../sign-in), not a password")
)

func helpdeskMigration(m store.Migration) bool { return migrate.Helpdesk(m.Source) }

// helpdeskSecret is what a help desk's sealed credentials hold: the team's OAuth client, the
// tokens, and the signed-in account a reconnect must sign in to again.
type helpdeskSecret struct {
	App       helpdesk.App    `json:"app"`
	Tokens    helpdesk.Tokens `json:"tokens"`
	Subdomain string          `json:"subdomain,omitempty"`
	Account   string          `json:"account"`
}

type signInSecret struct {
	App       helpdesk.App `json:"app"`
	Verifier  string       `json:"verifier,omitempty"`
	Subdomain string       `json:"subdomain,omitempty"`
}

// sourceStateDoc is migrations.source_state: where a help desk's listings stand.
type sourceStateDoc struct {
	Scan         helpdesk.Cursor `json:"scan"`
	ScanDone     bool            `json:"scan_done,omitempty"`
	Sync         helpdesk.Cursor `json:"sync"`
	SyncStarted  bool            `json:"sync_started,omitempty"`
	SignInResult string          `json:"sign_in_result,omitempty"`
	// AdminConsentURL is Microsoft's page where an admin consents for the organization.
	AdminConsentURL string `json:"admin_consent_url,omitempty"`
}

func readSourceState(raw []byte) sourceStateDoc {
	var d sourceStateDoc
	_ = json.Unmarshal(raw, &d)
	return d
}

func (s *Server) signInRedirectURL() string {
	return strings.TrimRight(s.auth.PublicURL, "/") + signInCallbackPath
}

// helpdeskConfig dials help desks and attachment hosts only after the address checks every
// migration connection follows.
func (s *Server) helpdeskConfig() helpdesk.Config {
	cfg := helpdesk.Config{RootCAs: s.migrations.RootCAs, Now: s.now, MaxBytes: max(s.attach.MaxBytes, s.migrations.MaxMessageBytes), Sleep: s.migrations.HelpdeskSleep}
	if s.migrations.HelpdeskDial != nil {
		cfg.Dial = s.migrations.HelpdeskDial
		return cfg
	}
	cfg.Dial = s.checkedDial
	return cfg
}

// checkedDial connects to a source only after the address checks every migration connection
// follows.
func (s *Server) checkedDial(ctx context.Context, _, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	ips, err := migrate.Addresses(ctx, s.migrations.Resolver, host, s.migrations.AllowPrivate)
	if err != nil {
		return nil, err
	}
	return migrate.DialAddresses(ctx, ips, n, 10*time.Second)
}

func (s *Server) helpdeskSource(source, subdomain string, token func() string, budget *helpdesk.Budget) helpdesk.Source {
	if source == migrate.SourceZendesk {
		return helpdesk.NewZendesk(s.helpdeskConfig(), subdomain, token, budget)
	}
	return helpdesk.NewHelpScout(s.helpdeskConfig(), token, budget)
}

func helpdeskHost(source, subdomain string) string {
	if source == migrate.SourceZendesk {
		return helpdesk.ZendeskHost(subdomain)
	}
	return helpdesk.HelpScoutAPIHost
}

func helpdeskScopes(source string) []string {
	if source == migrate.SourceZendesk {
		return []string{helpdesk.ZendeskScope}
	}
	return []string{}
}

// signInState names the workspace and the migration, so the callback finds its sign-in without
// looking across workspaces, and carries 32 random bytes of which only the hash is stored.
func signInState(ws, migration uuid.UUID) string {
	b := make([]byte, 64)
	copy(b, ws[:])
	copy(b[16:], migration[:])
	_, _ = rand.Read(b[32:])
	return base64.RawURLEncoding.EncodeToString(b)
}

func readSignInState(v string) (uuid.UUID, uuid.UUID, bool) {
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil || len(b) != 64 {
		return uuid.Nil(), uuid.Nil(), false
	}
	var ws, mid uuid.UUID
	copy(ws[:], b[:16])
	copy(mid[:], b[16:32])
	return ws, mid, true
}

func stateHash(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func (s *Server) StartMigrationSignIn(ctx context.Context, req oas.StartMigrationSignInRequestObject) (oas.StartMigrationSignInResponseObject, error) {
	p := principalFrom(ctx)
	if m, err := visibleMigration(ctx, s.st.Queries, p, req.MigrationId, false); err != nil {
		return nil, err
	} else if microsoftMigration(m) {
		return s.startMicrosoftSignIn(ctx, p, m, req.Body)
	}
	b := req.Body
	clientID, secret := strings.TrimSpace(b.ClientId), ""
	if b.ClientSecret != nil {
		secret = strings.TrimSpace(*b.ClientSecret)
	}
	if n := len(clientID); n < 1 || n > 200 {
		return nil, errValidation("client_id must be 1 to 200 characters")
	}
	if n := len(secret); n < 1 || n > 500 {
		return nil, errValidation("client_secret must be 1 to 500 characters")
	}
	subdomain := ""
	if b.Subdomain != nil {
		subdomain = strings.ToLower(strings.TrimSpace(*b.Subdomain))
	}
	var out oas.StartMigrationSignIn200JSONResponse
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		m, err := visibleMigration(ctx, q, p, req.MigrationId, true)
		if err != nil {
			return err
		}
		if !helpdeskMigration(m) {
			return errNotHelpdesk
		}
		switch m.State {
		case migrationDraft, migrationConnected, migrationNeedsAttention:
		default:
			return errMigrationState("a help desk signs in while the migration is a draft, connected or needs attention")
		}
		if m.Source == migrate.SourceZendesk {
			if !subdomainRe.MatchString(subdomain) {
				return errValidation("subdomain must be the account's subdomain, acme of acme.zendesk.com")
			}
		} else {
			subdomain = ""
		}
		host := helpdeskHost(m.Source, subdomain)
		if m.State == migrationNeedsAttention {
			cred, err := q.GetMigrationCredentials(ctx, store.GetMigrationCredentialsParams{WorkspaceID: m.WorkspaceID, MigrationID: m.ID})
			if err == nil && !strings.EqualFold(cred.Host, host) {
				return errMigrationState("reconnect the same account; start a new migration for another help desk")
			}
			if err != nil && !store.IsNotFound(err) {
				return err
			}
		}
		state := signInState(m.WorkspaceID, m.ID)
		sec := signInSecret{App: helpdesk.App{ClientID: clientID, ClientSecret: secret}, Subdomain: subdomain}
		authorize := helpdesk.HelpScoutAuthorizeURL(clientID, state)
		if m.Source == migrate.SourceZendesk {
			var challenge string
			sec.Verifier, challenge = helpdesk.PKCE()
			authorize = helpdesk.ZendeskAuthorizeURL(subdomain, clientID, s.signInRedirectURL(), state, challenge)
		}
		var keyID *uuid.UUID
		if p.isKey() {
			keyID = &p.keyID
		}
		now := s.now()
		if err := q.SetMigrationSignIn(ctx, store.SetMigrationSignInParams{
			WorkspaceID: m.WorkspaceID, MigrationID: m.ID, StateHash: stateHash(state),
			Sealed: s.secrets.Seal(mustJSON(sec), migrationSecretContext(m.WorkspaceID, m.ID)), Host: host,
			MemberID: p.memberRef(), ApiKeyID: keyID, ExpiresAt: now.Add(signInTTL), Now: now,
		}); err != nil {
			return err
		}
		if _, err := s.migrationAudit(ctx, q, p, m.ID, "sign_in_started", map[string]any{"host": host}); err != nil {
			return err
		}
		out = oas.StartMigrationSignIn200JSONResponse{AuthorizeUrl: authorize, RedirectUrl: s.signInRedirectURL(), ExpiresAt: now.Add(signInTTL)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// serveMigrationSignIn is where the help desk sends the browser back after its consent page.
func (s *Server) serveMigrationSignIn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	state := query.Get("state")
	ws, mid, ok := readSignInState(state)
	panel := strings.TrimRight(s.auth.PublicURL, "/")
	if state == microsoft.AdminConsentState {
		result := oas.SignInAccessDenied
		if strings.EqualFold(query.Get("admin_consent"), "true") {
			result = oas.SignInAdminConsented
		}
		http.Redirect(w, r, panel+"/settings/migrations?result="+string(result), http.StatusSeeOther)
		return
	}
	if !ok {
		http.Redirect(w, r, panel+"/settings/migrations?result="+string(oas.SignInExpired), http.StatusSeeOther)
		return
	}
	result := s.finishSignIn(ctx, r, ws, mid, state, query)
	target := panel + "/migrations/" + mid.String() + "/connect?" + url.Values{"result": {string(result)}, "workspace": {ws.String()}}.Encode()
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) finishSignIn(ctx context.Context, r *http.Request, ws, mid uuid.UUID, state string, query url.Values) oas.MigrationSignInResult {
	now := s.now()
	row, err := s.st.TakeMigrationSignIn(ctx, store.TakeMigrationSignInParams{WorkspaceID: ws, MigrationID: mid, StateHash: stateHash(state), Now: now})
	if err != nil {
		if !store.IsNotFound(err) {
			s.log.ErrorContext(ctx, "migration sign-in", slog.Any("error", err))
		}
		return oas.SignInExpired
	}
	if row.MemberID != nil && !s.sameMemberSession(ctx, r, ws, *row.MemberID) {
		return s.signInFailed(ctx, ws, mid, oas.SignInSessionMismatch)
	}
	m, err := s.st.GetMigration(ctx, store.GetMigrationParams{WorkspaceID: ws, ID: mid})
	if err == nil && microsoftMigration(m) {
		return s.finishMicrosoftSignIn(ctx, row, m, query)
	}
	if query.Get("error") != "" || query.Get("code") == "" {
		return s.signInFailed(ctx, ws, mid, oas.SignInAccessDenied)
	}
	if err != nil || !helpdeskMigration(m) || (m.State != migrationDraft && m.State != migrationConnected && m.State != migrationNeedsAttention) {
		return oas.SignInExpired
	}
	plain, err := s.secrets.Open(row.Sealed, migrationSecretContext(ws, mid))
	var sec signInSecret
	if err != nil || json.Unmarshal(plain, &sec) != nil {
		return s.signInFailed(ctx, ws, mid, oas.SignInExpired)
	}
	cctx, cancel := context.WithTimeout(ctx, signInTimeout)
	defer cancel()
	cfg := s.helpdeskConfig()
	code := query.Get("code")
	var tokens helpdesk.Tokens
	if m.Source == migrate.SourceZendesk {
		tokens, err = helpdesk.ZendeskExchange(cctx, cfg, sec.Subdomain, sec.App, code, s.signInRedirectURL(), sec.Verifier)
	} else {
		tokens, err = helpdesk.HelpScoutExchange(cctx, cfg, sec.App, code)
	}
	if err != nil {
		var te *helpdesk.TokenError
		switch {
		case errors.As(err, &te) && te.Status == http.StatusNotFound:
			return s.signInFailed(ctx, ws, mid, oas.SignInSubdomainNotFound)
		case errors.As(err, &te):
			return s.signInFailed(ctx, ws, mid, oas.SignInTokenFailed)
		}
		s.log.WarnContext(ctx, "migration sign-in token", slog.String("migration_id", mid.String()), slog.String("error", migrate.Scrub(err.Error(), sec.App.ClientSecret, code)))
		return s.signInFailed(ctx, ws, mid, oas.SignInSourceUnreachable)
	}
	if want := helpdeskScopes(m.Source); m.Source == migrate.SourceZendesk && !sameScopes(tokens.Scopes, want) {
		return s.signInFailed(ctx, ws, mid, oas.SignInScopeTooBroad)
	}
	budget := s.migrationBudget(m)
	src := s.helpdeskSource(m.Source, sec.Subdomain, func() string { return tokens.Access }, budget)
	me, err := src.Me(cctx)
	s.saveBudget(ctx, m, budget)
	if err != nil {
		return s.signInFailed(ctx, ws, mid, signInFailure(err))
	}
	if m.Source == migrate.SourceZendesk && !me.Admin {
		return s.signInFailed(ctx, ws, mid, oas.SignInNotAdmin)
	}
	folders, err := src.Folders(cctx)
	s.saveBudget(ctx, m, budget)
	if err != nil {
		return s.signInFailed(ctx, ws, mid, signInFailure(err))
	}
	secret := helpdeskSecret{App: sec.App, Tokens: tokens, Subdomain: sec.Subdomain, Account: me.ID}
	host := helpdeskHost(m.Source, sec.Subdomain)
	result := oas.SignInConnected
	err = s.inTx(ctx, ws, func(q *store.Queries, events *eventBatch) error {
		cur, err := q.LockMigration(ctx, store.LockMigrationParams{WorkspaceID: ws, ID: mid})
		if err != nil {
			return err
		}
		if cur.State != migrationDraft && cur.State != migrationConnected && cur.State != migrationNeedsAttention {
			result = oas.SignInExpired
			return nil
		}
		if cur.State == migrationNeedsAttention {
			old, err := q.GetMigrationCredentials(ctx, store.GetMigrationCredentialsParams{WorkspaceID: ws, MigrationID: mid})
			if err != nil && !store.IsNotFound(err) {
				return err
			}
			if err == nil {
				var prev helpdeskSecret
				plain, oerr := s.secrets.Open(old.Sealed, migrationSecretContext(ws, mid))
				if oerr != nil || json.Unmarshal(plain, &prev) != nil || prev.Account != me.ID || !strings.EqualFold(old.Host, host) {
					result = oas.SignInAccountMismatch
					return nil
				}
			}
		}
		if err := q.SetMigrationCredentials(ctx, store.SetMigrationCredentialsParams{
			WorkspaceID: ws, MigrationID: mid, Kind: string(oas.CredentialsOauth), Sealed: s.secrets.Seal(mustJSON(secret), migrationSecretContext(ws, mid)),
			Host: host, Port: helpdeskPort, Tls: string(oas.ImapTls), Scopes: append([]string{}, tokens.Scopes...), ExpiresAt: expiresAt(tokens), Now: s.now(),
		}); err != nil {
			return err
		}
		if cur.State != migrationNeedsAttention {
			if err := s.recordHelpdeskFolders(ctx, q, cur, folders); err != nil {
				return err
			}
		}
		doc := readSourceState(cur.SourceState)
		doc.SignInResult = string(oas.SignInConnected)
		if err := q.SetMigrationSourceState(ctx, store.SetMigrationSourceStateParams{WorkspaceID: ws, ID: mid, SourceState: mustJSON(doc)}); err != nil {
			return err
		}
		if _, err := q.AddMigrationAudit(ctx, store.AddMigrationAuditParams{
			WorkspaceID: ws, MigrationID: mid, Action: "signed_in", MemberID: row.MemberID, ApiKeyID: row.ApiKeyID,
			Detail: mustJSON(map[string]any{"host": host, "scopes": tokens.Scopes}), Now: s.now(),
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

func expiresAt(t helpdesk.Tokens) *time.Time {
	if t.Expires.IsZero() {
		return nil
	}
	return &t.Expires
}

func sameScopes(got, want []string) bool {
	a, b := slices.Clone(got), slices.Clone(want)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(slices.Compact(a), b)
}

// signInFailure names why a fresh token could not read the account.
func signInFailure(err error) oas.MigrationSignInResult {
	var auth *helpdesk.AuthError
	var se *helpdesk.StatusError
	switch {
	case errors.As(err, &auth):
		return oas.SignInTokenFailed
	case errors.As(err, &se) && se.Status == http.StatusForbidden:
		return oas.SignInPlanHasNoApi
	case errors.As(err, &se) && se.Status == http.StatusNotFound:
		return oas.SignInSubdomainNotFound
	}
	return oas.SignInSourceUnreachable
}

func (s *Server) signInFailed(ctx context.Context, ws, mid uuid.UUID, result oas.MigrationSignInResult) oas.MigrationSignInResult {
	return s.signInEnded(ctx, ws, mid, result, "")
}

// signInEnded records how a sign-in that stored nothing ended, with Microsoft's admin consent
// page when only an admin can consent.
func (s *Server) signInEnded(ctx context.Context, ws, mid uuid.UUID, result oas.MigrationSignInResult, adminConsent string) oas.MigrationSignInResult {
	err := s.inTx(ctx, ws, func(q *store.Queries, events *eventBatch) error {
		cur, err := q.LockMigration(ctx, store.LockMigrationParams{WorkspaceID: ws, ID: mid})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		doc := readSourceState(cur.SourceState)
		doc.SignInResult, doc.AdminConsentURL = string(result), adminConsent
		if err := q.SetMigrationSourceState(ctx, store.SetMigrationSourceStateParams{WorkspaceID: ws, ID: mid, SourceState: mustJSON(doc)}); err != nil {
			return err
		}
		if _, err := q.AddMigrationAudit(ctx, store.AddMigrationAuditParams{
			WorkspaceID: ws, MigrationID: mid, Action: "sign_in_failed", Detail: mustJSON(map[string]any{"result": result}), Now: s.now(),
		}); err != nil {
			return err
		}
		cur.SourceState = mustJSON(doc)
		return s.migrationEvent(ctx, q, events, cur, &cur.State)
	})
	if err != nil {
		s.log.ErrorContext(ctx, "migration sign-in result", slog.Any("error", err))
	}
	return result
}

// sameMemberSession tells whether the browser that came back is signed in as the member who
// started the sign-in.
func (s *Server) sameMemberSession(ctx context.Context, r *http.Request, ws, memberID uuid.UUID) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return false
	}
	sess, err := s.st.GetSessionByTokenHash(ctx, store.GetSessionByTokenHashParams{TokenHash: hashSecret(c.Value), ExpiresAt: s.now()})
	if err != nil {
		return false
	}
	mem, err := s.st.GetMember(ctx, store.GetMemberParams{WorkspaceID: ws, ID: memberID})
	return err == nil && mem.PersonID == sess.PersonID
}

// recordHelpdeskFolders stores a help desk's brands or mailboxes as the migration's folders, each
// an inbox of the source, named uniquely.
func (s *Server) recordHelpdeskFolders(ctx context.Context, q *store.Queries, m store.Migration, folders []helpdesk.Folder) error {
	listed := make([]listedFolder, 0, len(folders))
	names := make([]string, 0, len(folders))
	seen := map[string]bool{}
	for _, f := range folders {
		name := truncateRunes(strings.TrimSpace(f.Name), 900)
		if name == "" {
			name = "#" + f.ID
		}
		if seen[name] {
			name += " (" + f.ID + ")"
		}
		seen[name] = true
		listed = append(listed, listedFolder{Name: name, Kind: migrate.KindInbox, Messages: f.Count})
		names = append(names, name)
	}
	if err := s.recordFolders(ctx, q, m, listed); err != nil {
		return err
	}
	for i, f := range folders {
		var addr *string
		if f.Address != "" {
			addr = new(truncateRunes(f.Address, 320))
		}
		if err := q.SetMigrationFolderSource(ctx, store.SetMigrationFolderSourceParams{
			WorkspaceID: m.WorkspaceID, MigrationID: m.ID, Name: names[i], SourceRef: new(truncateRunes(f.ID, 200)), Address: addr,
		}); err != nil {
			return err
		}
	}
	return nil
}

// openHelpdesk reads a migration's sealed credentials, refreshing the access token when it expires
// within five minutes and sealing the new tokens at once.
func (s *Server) openHelpdesk(ctx context.Context, m store.Migration, budget *helpdesk.Budget) (helpdesk.Source, helpdeskSecret, error) {
	var sec helpdeskSecret
	cred, err := s.st.GetMigrationCredentials(ctx, store.GetMigrationCredentialsParams{WorkspaceID: m.WorkspaceID, MigrationID: m.ID})
	if store.IsNotFound(err) {
		return nil, sec, &helpdesk.AuthError{StatusError: helpdesk.StatusError{Status: http.StatusUnauthorized, Body: "no credentials are stored"}}
	}
	if err != nil {
		return nil, sec, err
	}
	plain, err := s.secrets.Open(cred.Sealed, migrationSecretContext(m.WorkspaceID, m.ID))
	if err != nil {
		return nil, sec, err
	}
	if err := json.Unmarshal(plain, &sec); err != nil {
		return nil, sec, err
	}
	if sec.Tokens.Refresh != "" && !sec.Tokens.Expires.IsZero() && s.now().Add(tokenRefreshAhead).After(sec.Tokens.Expires) {
		cctx, cancel := context.WithTimeout(ctx, signInTimeout)
		defer cancel()
		var t helpdesk.Tokens
		if m.Source == migrate.SourceZendesk {
			t, err = helpdesk.ZendeskRefresh(cctx, s.helpdeskConfig(), sec.Subdomain, sec.App, sec.Tokens.Refresh)
		} else {
			t, err = helpdesk.HelpScoutRefresh(cctx, s.helpdeskConfig(), sec.App, sec.Tokens.Refresh)
		}
		if err != nil {
			return nil, sec, err
		}
		if t.Refresh == "" {
			t.Refresh = sec.Tokens.Refresh
		}
		if len(t.Scopes) == 0 {
			t.Scopes = sec.Tokens.Scopes
		}
		if t.Scopes == nil {
			t.Scopes = []string{}
		}
		if m.Source == migrate.SourceZendesk && !sameScopes(t.Scopes, helpdeskScopes(m.Source)) {
			return nil, sec, &helpdesk.TokenError{Status: http.StatusForbidden, Code: "scope_too_broad"}
		}
		sec.Tokens = t
		if err := s.st.SetMigrationCredentials(ctx, store.SetMigrationCredentialsParams{
			WorkspaceID: m.WorkspaceID, MigrationID: m.ID, Kind: cred.Kind, Sealed: s.secrets.Seal(mustJSON(sec), migrationSecretContext(m.WorkspaceID, m.ID)),
			Host: cred.Host, Port: cred.Port, Tls: cred.Tls, Scopes: t.Scopes, ExpiresAt: expiresAt(t), Now: s.now(),
		}); err != nil {
			return nil, sec, err
		}
	}
	access := sec.Tokens.Access
	return s.helpdeskSource(m.Source, sec.Subdomain, func() string { return access }, budget), sec, nil
}

func (sec helpdeskSecret) secrets() []string {
	return []string{sec.Tokens.Access, sec.Tokens.Refresh, sec.App.ClientSecret}
}

// testHelpdesk runs a help desk's connection test: the stored token reads the signed-in user,
// then the brands or mailboxes.
func (s *Server) testHelpdesk(ctx context.Context, m store.Migration) (migrate.TestResult, []helpdesk.Folder) {
	ctx, cancel := context.WithTimeout(ctx, migrate.TestTimeout)
	defer cancel()
	res := migrate.TestResult{}
	fail := func(step, code string, err error, sec helpdeskSecret) (migrate.TestResult, []helpdesk.Folder) {
		res.Steps = append(res.Steps, migrate.TestStep{Step: step, Code: code, Detail: migrate.Scrub(err.Error(), sec.secrets()...)})
		return res, nil
	}
	budget := s.migrationBudget(m)
	defer s.saveBudget(ctx, m, budget)
	src, sec, err := s.openHelpdesk(ctx, m, budget)
	if err == nil {
		var me helpdesk.Person
		if me, err = src.Me(ctx); err == nil && m.Source == migrate.SourceZendesk && !me.Admin {
			return fail(migrate.StepSignIn, string(oas.TestCodeNotAdmin), errors.New("the signed-in Zendesk user is not an admin"), sec)
		}
	}
	if err != nil {
		code := string(oas.TestCodeAuthFailed)
		switch signInFailure(err) {
		case oas.SignInPlanHasNoApi:
			code = string(oas.TestCodePlanHasNoApi)
		case oas.SignInSubdomainNotFound:
			code = string(oas.TestCodeSubdomainNotFound)
		case oas.SignInSourceUnreachable:
			code = string(oas.TestCodeUnexpectedAnswer)
		}
		var limit *helpdesk.LimitError
		if errors.As(err, &limit) {
			code = string(oas.TestCodeThrottled)
		}
		return fail(migrate.StepSignIn, code, err, sec)
	}
	res.Steps = append(res.Steps, migrate.TestStep{Step: migrate.StepSignIn, OK: true})
	folders, err := src.Folders(ctx)
	if err != nil {
		return fail(migrate.StepFolders, string(oas.TestCodeUnexpectedAnswer), err, sec)
	}
	if len(folders) == 0 {
		return fail(migrate.StepFolders, string(oas.TestCodeNoFolders), errors.New("the help desk lists no brands or mailboxes"), sec)
	}
	res.Steps = append(res.Steps, migrate.TestStep{Step: migrate.StepFolders, OK: true})
	res.OK, res.Folders = true, len(folders)
	return res, folders
}

func (s *Server) testHelpdeskConnection(ctx context.Context, p principal, id uuid.UUID) (oas.TestMigrationConnectionResponseObject, error) {
	var (
		auditID int64
		m       store.Migration
		credAt  time.Time
	)
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		cur, err := visibleMigration(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if !testableMigration(cur.State) {
			return errMigrationState("the connection can be tested before the copy starts, or while it is paused or needs attention")
		}
		cred, err := q.GetMigrationCredentials(ctx, store.GetMigrationCredentialsParams{WorkspaceID: p.workspaceID, MigrationID: cur.ID})
		if store.IsNotFound(err) {
			return errMigrationState("sign in to the help desk before testing the connection")
		}
		if err != nil {
			return err
		}
		m, credAt = cur, cred.UpdatedAt
		auditID, err = s.claimMigrationTest(ctx, q, p, cur)
		return err
	})
	if err != nil {
		return nil, err
	}
	res, folders := s.testHelpdesk(context.WithoutCancel(ctx), m)
	failed := res.Failed()
	detail := map[string]any{"ok": res.OK}
	if !res.OK {
		detail["step"], detail["code"] = failed.Step, failed.Code
	}
	var out oas.Migration
	err = s.inTx(ctx, p.workspaceID, func(q *store.Queries, events *eventBatch) error {
		if err := q.SetMigrationAuditDetail(ctx, store.SetMigrationAuditDetailParams{WorkspaceID: p.workspaceID, ID: auditID, Detail: mustJSON(detail)}); err != nil {
			return err
		}
		cur, err := visibleMigration(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if res.OK && (cur.State == migrationDraft || cur.State == migrationConnected) {
			if err := s.recordHelpdeskFolders(ctx, q, cur, folders); err != nil {
				return err
			}
		}
		if res.OK && cur.State == migrationDraft {
			cred, err := q.GetMigrationCredentials(ctx, store.GetMigrationCredentialsParams{WorkspaceID: p.workspaceID, MigrationID: cur.ID})
			if err == nil && cred.UpdatedAt.Equal(credAt) {
				updated, err := q.SetMigrationState(ctx, store.SetMigrationStateParams{WorkspaceID: p.workspaceID, ID: cur.ID, State: migrationConnected, Now: s.now()})
				if err != nil {
					return err
				}
				if err := s.migrationChanged(ctx, q, events, updated, &cur.State); err != nil {
					return err
				}
				cur = updated
			} else if err != nil && !store.IsNotFound(err) {
				return err
			}
		}
		out, err = s.oneMigration(ctx, q, cur)
		return err
	})
	if err != nil {
		return nil, err
	}
	return testResultBody(res, out), nil
}

// migrationBudget is the request budget kept on the migration, which every reader of the help
// desk shares, a sign-in's and a connection test's too.
func (s *Server) migrationBudget(m store.Migration) *helpdesk.Budget {
	if t := readThrottle(m.Throttle); t.Requests != nil {
		return t.Requests
	}
	return &helpdesk.Budget{}
}

func (s *Server) saveBudget(ctx context.Context, m store.Migration, b *helpdesk.Budget) {
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		cur, err := q.LockMigration(ctx, store.LockMigrationParams{WorkspaceID: m.WorkspaceID, ID: m.ID})
		if err != nil {
			return err
		}
		t := readThrottle(cur.Throttle)
		t.Requests = b
		return q.SetMigrationThrottle(ctx, store.SetMigrationThrottleParams{WorkspaceID: m.WorkspaceID, ID: m.ID, Throttle: mustJSON(t)})
	})
	if err != nil {
		s.log.WarnContext(ctx, "migration budget", slog.Any("error", err))
	}
}
