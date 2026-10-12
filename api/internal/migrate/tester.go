package migrate

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/productdevbook/yuva/api/internal/migrate/imapread"
)

const (
	StepResolve  = "resolve"
	StepConnect  = "connect"
	StepTLS      = "tls"
	StepGreeting = "greeting"
	StepSignIn   = "sign_in"
	StepFolders  = "folders"

	CodeDNSFailed           = "dns_failed"
	CodeAddressNotAllowed   = "address_not_allowed"
	CodeConnectFailed       = "connect_failed"
	CodeTLSFailed           = "tls_failed"
	CodeNotIMAP             = "not_imap"
	CodeStartTLSUnavailable = "starttls_unavailable"
	CodeAuthFailed          = "auth_failed"
	CodeAppPasswordRequired = "app_password_required"
	CodeIMAPDisabled        = "imap_disabled"
	CodeBasicAuthDisabled   = "basic_auth_disabled"
	CodeWebLoginRequired    = "web_login_required"
	CodeTooManyConnections  = "too_many_connections"
	CodeThrottled           = "throttled"
	CodeNoFolders           = "no_folders"
	CodeExamineFailed       = "examine_failed"
	CodeTimeout             = "timeout"
	CodeUnexpectedAnswer    = "unexpected_answer"
	CodeXOAUTH2Failed       = "xoauth2_failed"
	CodeTokenRefreshFailed  = "token_refresh_failed"
	CodeScopeTooBroad       = "scope_too_broad"

	TestTimeout = 20 * time.Second
)

type TestStep struct {
	Step   string
	OK     bool
	Code   string
	Detail string
}

type TestResult struct {
	OK          bool
	Steps       []TestStep
	Folders     int
	Messages    int64
	Certificate string
	List        []FolderInfo
}

// FolderInfo is a folder the test saw with its message count, which stays 0 when the time ran out
// before it was counted.
type FolderInfo struct {
	Name     string
	Delim    string
	Kind     string
	Messages int64
}

func (r TestResult) Failed() TestStep {
	if len(r.Steps) == 0 || r.OK {
		return TestStep{}
	}
	return r.Steps[len(r.Steps)-1]
}

// Target is what a test connects to and signs in with: a password, or an OAuth access token sent
// with AUTHENTICATE XOAUTH2.
type Target struct {
	Preset            Preset
	Server            Server
	Username          string
	Password          string
	Token             string
	CertificateSHA256 string
}

// Tester runs connection tests. Dial, when set, replaces resolving and connecting, for tests.
type Tester struct {
	Resolver     Resolver
	AllowPrivate bool
	RootCAs      *x509.CertPool
	Timeout      time.Duration
	Dial         func(ctx context.Context, network, addr string) (net.Conn, error)
}

// Run goes through the steps a migration will take and stops at the first that fails.
func (t Tester) Run(ctx context.Context, target Target) TestResult {
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = TestTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var res TestResult
	pass := func(step string) { res.Steps = append(res.Steps, TestStep{Step: step, OK: true}) }
	fail := func(step, code, detail string) TestResult {
		if ctx.Err() != nil {
			code, detail = CodeTimeout, "the test ran out of its "+timeout.String()
		}
		res.Steps = append(res.Steps, TestStep{Step: step, Code: code, Detail: Scrub(detail, target.Username, target.Password, target.Token)})
		return res
	}
	host := target.Server.Host
	dial := t.Dial
	if dial == nil {
		ips, err := Addresses(ctx, t.Resolver, host, t.AllowPrivate)
		if errors.Is(err, ErrNotAllowed) {
			return fail(StepResolve, CodeAddressNotAllowed, host+" is on a private or local network")
		}
		if err != nil {
			return fail(StepResolve, CodeDNSFailed, host+" does not resolve")
		}
		dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return DialAddresses(ctx, ips, target.Server.Port, 10*time.Second)
		}
	}
	pass(StepResolve)
	r, err := imapread.Open(ctx, imapread.Config{
		Host: host, Port: target.Server.Port, TLS: imapread.TLSMode(target.Server.TLS),
		CertificateSHA256: target.CertificateSHA256, Dial: dial, RootCAs: t.RootCAs,
	})
	if err != nil {
		var ie *imapread.Error
		if !errors.As(err, &ie) {
			return fail(StepConnect, CodeConnectFailed, err.Error())
		}
		switch ie.Step {
		case imapread.StepConnect:
			return fail(StepConnect, CodeConnectFailed, connectDetail(host, target.Server.Port, ie.Err))
		case imapread.StepTLS:
			pass(StepConnect)
			if errors.Is(ie.Err, imapread.ErrNoStartTLS) {
				return fail(StepTLS, CodeStartTLSUnavailable, host+" offers no STARTTLS")
			}
			res.Certificate = ie.Certificate
			return fail(StepTLS, CodeTLSFailed, tlsDetail(ie))
		default:
			pass(StepConnect)
			pass(StepTLS)
			var se *imapread.ServerError
			if errors.As(ie.Err, &se) {
				code, detail := ClassifyAnswer(se, host)
				if code == CodeAuthFailed {
					code = CodeUnexpectedAnswer
				}
				return fail(StepGreeting, code, detail)
			}
			return fail(StepGreeting, CodeNotIMAP, host+" does not answer as an IMAP server")
		}
	}
	defer r.Close()
	pass(StepConnect)
	pass(StepTLS)
	pass(StepGreeting)
	if target.Token != "" {
		if err := r.LoginXOAUTH2(ctx, target.Username, target.Token); err != nil {
			var se *imapread.ServerError
			switch {
			case errors.As(err, &se):
				code, detail := ClassifyXOAUTH2(se)
				return fail(StepSignIn, code, detail)
			case errors.Is(err, imapread.ErrNoXOAUTH2):
				return fail(StepSignIn, CodeXOAUTH2Failed, host+" offers no XOAUTH2 sign-in")
			}
			return fail(StepSignIn, CodeUnexpectedAnswer, err.Error())
		}
	} else if err := r.Login(ctx, target.Username, target.Password); err != nil {
		var se *imapread.ServerError
		if errors.As(err, &se) {
			code, detail := ClassifyAnswer(se, host)
			return fail(StepSignIn, code, detail)
		}
		if errors.Is(err, imapread.ErrNoSignIn) {
			return fail(StepSignIn, CodeBasicAuthDisabled, host+" accepts no password sign-in")
		}
		return fail(StepSignIn, CodeUnexpectedAnswer, err.Error())
	}
	pass(StepSignIn)
	folders, err := r.Folders(ctx)
	if err != nil {
		var se *imapread.ServerError
		if errors.As(err, &se) {
			if code, detail := ClassifyAnswer(se, host); code == CodeXOAUTH2Failed {
				return fail(StepFolders, code, detail)
			}
		}
		return fail(StepFolders, CodeNoFolders, answerText(err))
	}
	if len(folders) == 0 {
		return fail(StepFolders, CodeNoFolders, "the mailbox lists no folders")
	}
	inbox := "INBOX"
	for _, f := range folders {
		if strings.EqualFold(f.Name, "INBOX") {
			inbox = f.Name
		}
	}
	if _, err := r.Examine(ctx, inbox); err != nil {
		return fail(StepFolders, CodeExamineFailed, answerText(err))
	}
	res.Folders = len(folders)
	gmail := target.Preset.ID == PresetGmail || target.Preset.ID == PresetGoogleWorkspace
	for _, f := range folders {
		if f.Selectable() {
			res.List = append(res.List, FolderInfo{Name: f.Name, Delim: f.Delim, Kind: FolderKind(f, gmail)})
		}
	}
	for _, read := range []bool{true, false} {
		for i, info := range res.List {
			if d, ok := ctx.Deadline(); DefaultSelected(target.Preset, info.Kind) != read || (ok && time.Until(d) < 2*time.Second) {
				continue
			}
			if st, err := r.Status(ctx, info.Name); err == nil {
				res.List[i].Messages = int64(st.Messages)
				if read {
					res.Messages += int64(st.Messages)
				}
			}
		}
	}
	pass(StepFolders)
	res.OK = true
	return res
}

func answerText(err error) string {
	var se *imapread.ServerError
	if errors.As(err, &se) {
		return se.Error()
	}
	return err.Error()
}

func connectDetail(host string, port int, err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return fmt.Sprintf("%s refuses connections on port %d", host, port)
	case errors.Is(err, os.ErrDeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return fmt.Sprintf("nothing answers on port %d of %s (timed out)", port, host)
	}
	return fmt.Sprintf("nothing answers on port %d of %s", port, host)
}

func tlsDetail(ie *imapread.Error) string {
	var ue x509.UnknownAuthorityError
	var he x509.HostnameError
	var ce x509.CertificateInvalidError
	switch {
	case errors.As(ie.Err, &he):
		return "the certificate is not valid for " + he.Host
	case errors.As(ie.Err, &ue):
		return "the certificate is not signed by a trusted authority"
	case errors.As(ie.Err, &ce):
		return "the certificate is not valid: " + ce.Error()
	}
	return "TLS failed: " + ie.Err.Error()
}

var (
	appPasswordRe = regexp.MustCompile(`(?i)application-specific password|app password required|app-specific password`)
	webLoginRe    = regexp.MustCompile(`(?i)log ?in via your web browser|web login required|sign in (via|through|with) (your|a) (web )?browser`)
	imapOffRe     = regexp.MustCompile(`(?i)imap (is |access is |access )?(disabled|not enabled|turned off)|enable imap|imap access (is )?not allowed|imap is not enabled`)
	connectionsRe = regexp.MustCompile(`(?i)too many (simultaneous )?connections|maximum number of connections|connection limit`)
	throttledRe   = regexp.MustCompile(`(?i)throttl|rate limit|too many (login|requests|commands)|bandwidth limit|try again later|server busy|temporarily unavailable`)
	notConnected  = regexp.MustCompile(`(?i)authenticated but not connected`)
)

// ClassifyAnswer maps a server's answer to a sign-in or greeting to a test code and a detail
// that names the answer.
func ClassifyAnswer(se *imapread.ServerError, host string) (string, string) {
	detail := se.Error()
	text := se.Code + " " + se.Text
	code := strings.ToUpper(se.Code)
	switch {
	case notConnected.MatchString(text):
		return CodeXOAUTH2Failed, detail
	case appPasswordRe.MatchString(text):
		return CodeAppPasswordRequired, detail
	case webLoginRe.MatchString(text):
		return CodeWebLoginRequired, detail
	case imapOffRe.MatchString(text):
		return CodeIMAPDisabled, detail
	case connectionsRe.MatchString(text):
		return CodeTooManyConnections, detail
	case code == "THROTTLED" || code == "LIMIT" || throttledRe.MatchString(text):
		return CodeThrottled, detail
	case MicrosoftHost(host):
		return CodeBasicAuthDisabled, detail
	case code == "AUTHENTICATIONFAILED" || code == "AUTHORIZATIONFAILED" || code == "EXPIRED" || se.Type == "NO":
		return CodeAuthFailed, detail
	}
	return CodeUnexpectedAnswer, detail
}

// ClassifyXOAUTH2 maps the answer to AUTHENTICATE XOAUTH2: a limit, or the mailbox refusing the
// token, which on Microsoft 365 usually means IMAP is turned off for the mailbox.
func ClassifyXOAUTH2(se *imapread.ServerError) (string, string) {
	detail := se.Error()
	text := se.Code + " " + se.Text
	code := strings.ToUpper(se.Code)
	switch {
	case connectionsRe.MatchString(text):
		return CodeTooManyConnections, detail
	case code == "THROTTLED" || code == "LIMIT" || throttledRe.MatchString(text):
		return CodeThrottled, detail
	}
	return CodeXOAUTH2Failed, detail
}

var base64Run = regexp.MustCompile(`[A-Za-z0-9+/]{16,}={0,2}`)

// Scrub removes secrets, and any base64 run long enough to carry one, from a server's answer
// before it is stored or shown, and keeps it to one short printable line.
func Scrub(s string, secrets ...string) string {
	for _, sec := range secrets {
		if len(sec) >= 4 {
			s = strings.ReplaceAll(s, sec, "[removed]")
		}
	}
	s = base64Run.ReplaceAllStringFunc(s, func(m string) string {
		mixed := strings.ContainsAny(m, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") && strings.ContainsAny(m, "abcdefghijklmnopqrstuvwxyz") &&
			strings.ContainsAny(m, "0123456789")
		if mixed || strings.Contains(m, "+") || strings.HasSuffix(m, "=") {
			return "[removed]"
		}
		return m
	})
	s = strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return ' '
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 300 {
		s = string(r[:300]) + "…"
	}
	return s
}
