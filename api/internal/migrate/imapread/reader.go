// Package imapread reads a source mailbox over IMAP and can do nothing else. It is the only
// package that may import the IMAP client: mailboxes are opened with EXAMINE, bodies fetched with
// BODY.PEEK, and the wire guard under the client refuses any other command before it is sent.
package imapread

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"
)

type TLSMode string

const (
	ImplicitTLS TLSMode = "tls"
	StartTLS    TLSMode = "starttls"
)

type Step string

const (
	StepConnect  Step = "connect"
	StepTLS      Step = "tls"
	StepGreeting Step = "greeting"
	StepSignIn   Step = "sign_in"
	StepRead     Step = "read"
)

// Error names the step that failed. Certificate is the server certificate's SHA-256 when TLS
// verification failed.
type Error struct {
	Step        Step
	Err         error
	Certificate string
}

func (e *Error) Error() string { return "imapread: " + string(e.Step) + ": " + e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// ServerError is a NO, BAD or BYE answer of the server.
type ServerError struct {
	Type string
	Code string
	Text string
}

func (e *ServerError) Error() string {
	if e.Code != "" {
		return e.Type + " [" + e.Code + "] " + e.Text
	}
	return e.Type + " " + e.Text
}

var (
	ErrNotIMAP          = errors.New("the server does not speak IMAP")
	ErrNoStartTLS       = errors.New("the server offers no STARTTLS")
	ErrPreauthPlaintext = errors.New("the server signed in before TLS")
	ErrNoSignIn         = errors.New("the server offers no password sign-in")
	ErrNoXOAUTH2        = errors.New("the server offers no XOAUTH2 sign-in")
)

// TokenRefusedError is the server's answer to AUTHENTICATE XOAUTH2 that refused the token.
type TokenRefusedError struct{ *ServerError }

func (e *TokenRefusedError) Unwrap() error { return e.ServerError }

type Config struct {
	Host string
	Port int
	TLS  TLSMode
	// CertificateSHA256, when set, is the only certificate accepted, whatever its name or issuer.
	CertificateSHA256 string
	// Dial opens the TCP connection; the caller checks the address it connects to.
	Dial func(ctx context.Context, network, address string) (net.Conn, error)
	// RootCAs replaces the system roots, for tests.
	RootCAs *x509.CertPool
}

// Reader is one read-only IMAP session.
type Reader struct {
	client *imapclient.Client
	raw    net.Conn
	guard  *guard
	caps   map[string]bool
}

type Folder struct {
	Name  string
	Delim string
	Attrs []string
}

func (f Folder) Has(attr string) bool {
	for _, a := range f.Attrs {
		if strings.EqualFold(a, attr) {
			return true
		}
	}
	return false
}

// Selectable reports whether the folder can be opened.
func (f Folder) Selectable() bool { return !f.Has(`\Noselect`) && !f.Has(`\NonExistent`) }

type Status struct {
	Messages      uint32
	UIDNext       uint32
	UIDValidity   uint32
	HighestModSeq uint64
}

type Header struct {
	UID          uint32
	InternalDate time.Time
	Size         int64
	Flags        []string
	Header       []byte
	Parts        []Part
}

// Part is a leaf of a message's structure, as the server describes it; Size is encoded.
type Part struct {
	Type     string
	Size     int64
	Encoding string
	Filename string
	ID       string
	Inline   bool
}

type Message struct {
	UID          uint32
	InternalDate time.Time
	Raw          []byte
}

// Open connects, sets up TLS and reads the greeting.
func Open(ctx context.Context, cfg Config) (*Reader, error) {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	raw, err := cfg.Dial(ctx, "tcp", addr)
	if err != nil {
		return nil, &Error{Step: StepConnect, Err: err}
	}
	stop := closeOn(ctx, raw)
	defer stop()
	if d, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(d)
	}
	r := &Reader{raw: raw}
	tlsCfg, cert := tlsConfig(cfg)
	var conn net.Conn
	switch cfg.TLS {
	case StartTLS:
		conn, err = r.startTLS(raw, tlsCfg, cert)
	default:
		tc := tls.Client(raw, tlsCfg)
		if err = tc.HandshakeContext(ctx); err != nil {
			err = &Error{Step: StepTLS, Err: err, Certificate: *cert}
		}
		r.guard = newGuard(tc)
		conn = r.guard
	}
	if err != nil {
		raw.Close()
		return nil, err
	}
	r.client = imapclient.New(conn, nil)
	if err := r.client.WaitGreeting(); err != nil {
		raw.Close()
		return nil, &Error{Step: StepGreeting, Err: notIMAP(err)}
	}
	caps := r.client.Caps()
	if !caps.Has(imap.CapIMAP4rev1) && !caps.Has(imap.CapIMAP4rev2) {
		raw.Close()
		return nil, &Error{Step: StepGreeting, Err: ErrNotIMAP}
	}
	r.caps = map[string]bool{}
	for c := range caps {
		r.caps[strings.ToUpper(string(c))] = true
	}
	_ = raw.SetDeadline(time.Time{})
	return r, nil
}

func notIMAP(err error) error {
	var ie *imap.Error
	if errors.As(err, &ie) {
		return serverError(err)
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrNotIMAP, err)
}

func tlsConfig(cfg Config) (*tls.Config, *string) {
	seen := new(string)
	pinned := strings.ToLower(strings.ReplaceAll(cfg.CertificateSHA256, ":", ""))
	return &tls.Config{
		ServerName:         cfg.Host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		NextProtos:         []string{"imap"},
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("the server sent no certificate")
			}
			sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
			*seen = hex.EncodeToString(sum[:])
			if pinned != "" {
				if *seen != pinned {
					return errors.New("the server's certificate is not the trusted one")
				}
				return nil
			}
			opts := x509.VerifyOptions{DNSName: cfg.Host, Roots: cfg.RootCAs, Intermediates: x509.NewCertPool()}
			for _, c := range cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(c)
			}
			_, err := cs.PeerCertificates[0].Verify(opts)
			return err
		},
	}, seen
}

// startTLS upgrades a plain connection before the client sees it, so the guard always sits on
// the clear text side of TLS.
func (r *Reader) startTLS(raw net.Conn, tlsCfg *tls.Config, cert *string) (net.Conn, error) {
	br := bufio.NewReader(raw)
	line, err := readLine(br)
	if err != nil {
		return nil, &Error{Step: StepGreeting, Err: notIMAP(err)}
	}
	switch {
	case strings.HasPrefix(line, "* OK"):
	case strings.HasPrefix(line, "* PREAUTH"):
		return nil, &Error{Step: StepGreeting, Err: ErrPreauthPlaintext}
	case strings.HasPrefix(line, "* BYE"):
		return nil, &Error{Step: StepGreeting, Err: &ServerError{Type: "BYE", Text: strings.TrimSpace(strings.TrimPrefix(line, "* BYE"))}}
	default:
		return nil, &Error{Step: StepGreeting, Err: ErrNotIMAP}
	}
	plain := newGuard(raw)
	if _, err := plain.Write([]byte("Y0 STARTTLS\r\n")); err != nil {
		return nil, &Error{Step: StepGreeting, Err: err}
	}
	for {
		line, err := readLine(br)
		if err != nil {
			return nil, &Error{Step: StepGreeting, Err: err}
		}
		if strings.HasPrefix(line, "* ") {
			continue
		}
		if !strings.HasPrefix(line, "Y0 OK") {
			return nil, &Error{Step: StepTLS, Err: ErrNoStartTLS}
		}
		break
	}
	if br.Buffered() > 0 {
		return nil, &Error{Step: StepTLS, Err: errors.New("the server sent data before TLS started")}
	}
	tc := tls.Client(raw, tlsCfg)
	if err := tc.Handshake(); err != nil {
		return nil, &Error{Step: StepTLS, Err: err, Certificate: *cert}
	}
	r.guard = newGuard(tc)
	return &greetedConn{Conn: r.guard, r: io.MultiReader(strings.NewReader("* OK TLS started\r\n"), tc)}, nil
}

func readLine(br *bufio.Reader) (string, error) {
	var b []byte
	for {
		chunk, isPrefix, err := br.ReadLine()
		if err != nil {
			return "", err
		}
		b = append(b, chunk...)
		if len(b) > 8192 {
			return "", ErrNotIMAP
		}
		if !isPrefix {
			return string(b), nil
		}
	}
}

// greetedConn hands the client the greeting the server already sent before STARTTLS.
type greetedConn struct {
	net.Conn
	r io.Reader
}

func (c *greetedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

func closeOn(ctx context.Context, c net.Conn) func() bool {
	return context.AfterFunc(ctx, func() { _ = c.Close() })
}

// bind ties one call to ctx: its deadline, and closing the connection when ctx ends first.
func (r *Reader) bind(ctx context.Context) func() {
	if d, ok := ctx.Deadline(); ok {
		_ = r.raw.SetDeadline(d)
	}
	stop := closeOn(ctx, r.raw)
	return func() {
		stop()
		_ = r.raw.SetDeadline(time.Time{})
	}
}

func serverError(err error) error {
	var ie *imap.Error
	if errors.As(err, &ie) {
		return &ServerError{Type: string(ie.Type), Code: string(ie.Code), Text: ie.Text}
	}
	return err
}

func (r *Reader) fail(step Step, err error) error {
	return &Error{Step: step, Err: serverError(err)}
}

// Capabilities are the server's, upper case.
func (r *Reader) Capabilities() map[string]bool { return r.caps }

// Commands lists the commands sent so far, by name.
func (r *Reader) Commands() []string { return r.guard.Commands() }

// Login signs in with a user name and password, with LOGIN or, where the server disables it,
// AUTHENTICATE PLAIN.
func (r *Reader) Login(ctx context.Context, username, password string) error {
	defer r.bind(ctx)()
	var err error
	switch {
	case !r.caps["LOGINDISABLED"]:
		err = r.client.Login(username, password).Wait()
	case r.caps["AUTH=PLAIN"]:
		err = r.client.Authenticate(sasl.NewPlainClient("", username, password))
	default:
		return &Error{Step: StepSignIn, Err: ErrNoSignIn}
	}
	if err != nil {
		return r.fail(StepSignIn, err)
	}
	r.readCaps()
	return nil
}

func (r *Reader) readCaps() {
	caps := r.client.Caps()
	r.caps = map[string]bool{}
	for c := range caps {
		r.caps[strings.ToUpper(string(c))] = true
	}
}

// LoginXOAUTH2 signs in with an OAuth access token for username (SASL XOAUTH2).
func (r *Reader) LoginXOAUTH2(ctx context.Context, username, token string) error {
	defer r.bind(ctx)()
	if !r.caps["AUTH=XOAUTH2"] {
		return &Error{Step: StepSignIn, Err: ErrNoXOAUTH2}
	}
	if err := r.client.Authenticate(xoauth2{user: username, token: token}); err != nil {
		var se *ServerError
		if errors.As(serverError(err), &se) {
			return &Error{Step: StepSignIn, Err: &TokenRefusedError{se}}
		}
		return r.fail(StepSignIn, err)
	}
	r.readCaps()
	return nil
}

type xoauth2 struct{ user, token string }

func (x xoauth2) Start() (string, []byte, error) {
	return "XOAUTH2", []byte("user=" + x.user + "\x01auth=Bearer " + x.token + "\x01\x01"), nil
}

// Next answers the error the server sends as a challenge with the empty response that ends the
// exchange, so the server's final answer says why.
func (x xoauth2) Next([]byte) ([]byte, error) { return []byte{}, nil }

// Folders lists every folder with its attributes, special use included where the server has it.
func (r *Reader) Folders(ctx context.Context) ([]Folder, error) {
	defer r.bind(ctx)()
	var opts *imap.ListOptions
	if r.caps["SPECIAL-USE"] {
		opts = &imap.ListOptions{ReturnSpecialUse: true}
	}
	rows, err := r.client.List("", "*", opts).Collect()
	if err != nil {
		return nil, r.fail(StepRead, err)
	}
	out := make([]Folder, 0, len(rows))
	for _, row := range rows {
		f := Folder{Name: row.Mailbox}
		if row.Delim != 0 {
			f.Delim = string(row.Delim)
		}
		for _, a := range row.Attrs {
			f.Attrs = append(f.Attrs, string(a))
		}
		out = append(out, f)
	}
	return out, nil
}

// Status reads a folder's counters without opening it.
func (r *Reader) Status(ctx context.Context, name string) (Status, error) {
	defer r.bind(ctx)()
	opts := &imap.StatusOptions{NumMessages: true, UIDNext: true, UIDValidity: true, HighestModSeq: r.caps["CONDSTORE"]}
	data, err := r.client.Status(name, opts).Wait()
	if err != nil {
		return Status{}, r.fail(StepRead, err)
	}
	out := Status{UIDNext: uint32(data.UIDNext), UIDValidity: data.UIDValidity, HighestModSeq: data.HighestModSeq}
	if data.NumMessages != nil {
		out.Messages = *data.NumMessages
	}
	return out, nil
}

// Examine opens a folder read-only.
func (r *Reader) Examine(ctx context.Context, name string) (Status, error) {
	defer r.bind(ctx)()
	data, err := r.client.Select(name, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return Status{}, r.fail(StepRead, err)
	}
	return Status{Messages: data.NumMessages, UIDNext: uint32(data.UIDNext), UIDValidity: data.UIDValidity, HighestModSeq: data.HighestModSeq}, nil
}

// SearchUIDs lists the UIDs of the open folder, all of them or those received since a day.
func (r *Reader) SearchUIDs(ctx context.Context, since time.Time) ([]uint32, error) {
	defer r.bind(ctx)()
	criteria := &imap.SearchCriteria{}
	if !since.IsZero() {
		criteria.Since = since
	}
	data, err := r.client.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, r.fail(StepRead, err)
	}
	set, ok := data.All.(imap.UIDSet)
	if data.All == nil {
		return nil, nil
	}
	if !ok || set.Dynamic() {
		return nil, &Error{Step: StepRead, Err: errors.New("the server answered SEARCH with a dynamic set")}
	}
	uids, _ := set.Nums()
	out := make([]uint32, len(uids))
	for i, u := range uids {
		out[i] = uint32(u)
	}
	return out, nil
}

func uidSet(uids []uint32) imap.UIDSet {
	var s imap.UIDSet
	for _, u := range uids {
		s.AddNum(imap.UID(u))
	}
	return s
}

// FetchHeaders reads the named header fields of messages of the open folder, with their date,
// size, flags and structure, without marking anything.
func (r *Reader) FetchHeaders(ctx context.Context, uids []uint32, fields []string, each func(Header) error) error {
	if len(uids) == 0 {
		return nil
	}
	defer r.bind(ctx)()
	opts := &imap.FetchOptions{
		UID: true, InternalDate: true, RFC822Size: true, Flags: true, BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
		BodySection: []*imap.FetchItemBodySection{{Specifier: imap.PartSpecifierHeader, HeaderFields: fields, Peek: true}},
	}
	cmd := r.client.Fetch(uidSet(uids), opts)
	defer cmd.Close()
	for msg := cmd.Next(); msg != nil; msg = cmd.Next() {
		buf, err := msg.Collect()
		if err != nil {
			return r.fail(StepRead, err)
		}
		h := Header{UID: uint32(buf.UID), InternalDate: buf.InternalDate, Size: buf.RFC822Size}
		for _, f := range buf.Flags {
			h.Flags = append(h.Flags, string(f))
		}
		for _, b := range buf.BodySection {
			h.Header = b.Bytes
		}
		h.Parts = leafParts(buf.BodyStructure)
		if err := each(h); err != nil {
			return err
		}
	}
	if err := cmd.Close(); err != nil {
		return r.fail(StepRead, err)
	}
	return nil
}

func leafParts(bs imap.BodyStructure) []Part {
	if bs == nil {
		return nil
	}
	var out []Part
	bs.Walk(func(_ []int, p imap.BodyStructure) bool {
		single, ok := p.(*imap.BodyStructureSinglePart)
		if !ok {
			return true
		}
		part := Part{Type: single.MediaType(), Size: int64(single.Size), Encoding: strings.ToLower(single.Encoding),
			Filename: single.Filename(), ID: strings.Trim(single.ID, "<> ")}
		if d := single.Disposition(); d != nil {
			part.Inline = strings.EqualFold(d.Value, "inline")
		}
		out = append(out, part)
		return true
	})
	return out
}

// FetchRaw reads whole messages of the open folder without marking them as seen. A message
// larger than maxBytes is skipped with a nil Raw.
func (r *Reader) FetchRaw(ctx context.Context, uids []uint32, maxBytes int64, each func(Message) error) error {
	if len(uids) == 0 {
		return nil
	}
	defer r.bind(ctx)()
	opts := &imap.FetchOptions{
		UID: true, InternalDate: true,
		BodySection: []*imap.FetchItemBodySection{{Peek: true}},
	}
	cmd := r.client.Fetch(uidSet(uids), opts)
	defer cmd.Close()
	for msg := cmd.Next(); msg != nil; msg = cmd.Next() {
		var m Message
		for item := msg.Next(); item != nil; item = msg.Next() {
			switch it := item.(type) {
			case imapclient.FetchItemDataUID:
				m.UID = uint32(it.UID)
			case imapclient.FetchItemDataInternalDate:
				m.InternalDate = it.Time
			case imapclient.FetchItemDataBodySection:
				if it.Literal == nil || it.Literal.Size() > maxBytes {
					continue
				}
				raw := make([]byte, it.Literal.Size())
				if _, err := io.ReadFull(it.Literal, raw); err != nil {
					return r.fail(StepRead, err)
				}
				m.Raw = raw
			}
		}
		if err := each(m); err != nil {
			return err
		}
	}
	if err := cmd.Close(); err != nil {
		return r.fail(StepRead, err)
	}
	return nil
}

// Close says LOGOUT when it can and closes the connection.
func (r *Reader) Close() error {
	done := make(chan struct{})
	go func() {
		_ = r.client.Logout().Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	return r.client.Close()
}
