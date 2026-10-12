package imapread

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// imapTarget reads YUVA_TEST_IMAP_URL: imaps://:password@host:port for TLS, imap://... for
// STARTTLS, against the migrate-test Dovecot with the fixtures of testdata/gen.
func imapTarget(t *testing.T) (*url.URL, string) {
	t.Helper()
	raw := os.Getenv("YUVA_TEST_IMAP_URL")
	if raw == "" {
		t.Skip("YUVA_TEST_IMAP_URL is not set")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "imaps" && u.Scheme != "imap") {
		t.Fatalf("YUVA_TEST_IMAP_URL must be imaps:// or imap://: %v", err)
	}
	pass, _ := u.User.Password()
	return u, pass
}

type folderSnapshot struct {
	UIDValidity   uint32
	UIDNext       imap.UID
	Messages      uint32
	HighestModSeq uint64
	Rows          []string
}

// snapshot reads every folder's counters and every message's UID, flags and modseq through a
// connection of its own with CONDSTORE on, so any change to the mailbox shows.
func snapshot(t *testing.T, u *url.URL, user, pass string) map[string]folderSnapshot {
	t.Helper()
	opts := &imapclient.Options{TLSConfig: &tls.Config{InsecureSkipVerify: true}}
	var (
		c   *imapclient.Client
		err error
	)
	if u.Scheme == "imaps" {
		c, err = imapclient.DialTLS(u.Host, opts)
	} else {
		c, err = imapclient.DialStartTLS(u.Host, opts)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Login(user, pass).Wait(); err != nil {
		t.Fatal(err)
	}
	list, err := c.List("", "*", nil).Collect()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]folderSnapshot{}
	for _, f := range list {
		st, err := c.Status(f.Mailbox, &imap.StatusOptions{NumMessages: true, UIDNext: true, UIDValidity: true, HighestModSeq: true}).Wait()
		if err != nil {
			t.Fatal(err)
		}
		snap := folderSnapshot{UIDValidity: st.UIDValidity, UIDNext: st.UIDNext, Messages: *st.NumMessages, HighestModSeq: st.HighestModSeq}
		if *st.NumMessages > 0 {
			if _, err := c.Select(f.Mailbox, &imap.SelectOptions{ReadOnly: true, CondStore: true}).Wait(); err != nil {
				t.Fatal(err)
			}
			rows, err := c.Fetch(seqAll(), &imap.FetchOptions{UID: true, Flags: true, ModSeq: true}).Collect()
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range rows {
				flags := make([]string, len(r.Flags))
				for i, fl := range r.Flags {
					flags[i] = string(fl)
				}
				slices.Sort(flags)
				snap.Rows = append(snap.Rows, fmt.Sprintf("%d %v %d", r.UID, flags, r.ModSeq))
			}
			slices.Sort(snap.Rows)
		}
		out[f.Mailbox] = snap
	}
	_ = c.Logout().Wait()
	return out
}

func seqAll() imap.SeqSet {
	var s imap.SeqSet
	s.AddRange(1, 0)
	return s
}

func openDovecot(t *testing.T, ctx context.Context, u *url.URL, pin string) (*Reader, error) {
	host, port, _ := net.SplitHostPort(u.Host)
	var p int
	fmt.Sscan(port, &p)
	mode := ImplicitTLS
	if u.Scheme == "imap" {
		mode = StartTLS
	}
	return Open(ctx, Config{
		Host: host, Port: p, TLS: mode, CertificateSHA256: pin,
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		},
	})
}

func TestDovecotSourceUnchanged(t *testing.T) {
	u, pass := imapTarget(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_, err := openDovecot(t, ctx, u, "")
	var ie *Error
	if !errors.As(err, &ie) || ie.Step != StepTLS || len(ie.Certificate) != 64 {
		t.Fatalf("an unknown self-signed certificate was accepted: %v", err)
	}
	pin := ie.Certificate
	if _, err := openDovecot(t, ctx, u, strings.Repeat("0", 64)); !errors.As(err, &ie) || ie.Step != StepTLS {
		t.Fatalf("a wrong pin was accepted: %v", err)
	}
	for _, user := range []string{"acme", "yerel"} {
		before := snapshot(t, u, user, pass)
		if len(before) < 5 {
			t.Fatalf("%s: only %d folders; generate the fixtures first", user, len(before))
		}
		r, err := openDovecot(t, ctx, u, pin)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Login(ctx, user, "wrong-"+pass); err == nil {
			t.Fatal("a wrong password signed in")
		} else {
			var se *ServerError
			if !errors.As(err, &se) || se.Code != "AUTHENTICATIONFAILED" {
				t.Fatalf("wrong password: %v", err)
			}
		}
		if err := r.Login(ctx, user, pass); err != nil {
			t.Fatal(err)
		}
		folders, err := r.Folders(ctx)
		if err != nil {
			t.Fatal(err)
		}
		fetched := 0
		for _, f := range folders {
			if !f.Selectable() {
				continue
			}
			st, err := r.Status(ctx, f.Name)
			if err != nil {
				t.Fatal(err)
			}
			ex, err := r.Examine(ctx, f.Name)
			if err != nil {
				t.Fatal(err)
			}
			if ex.UIDValidity != st.UIDValidity || ex.Messages != st.Messages {
				t.Fatalf("%s: EXAMINE %+v, STATUS %+v", f.Name, ex, st)
			}
			uids, err := r.SearchUIDs(ctx, time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			if len(uids) != int(st.Messages) {
				t.Fatalf("%s: %d UIDs for %d messages", f.Name, len(uids), st.Messages)
			}
			err = r.FetchHeaders(ctx, uids, []string{"Message-ID", "In-Reply-To", "References", "From", "Date", "Subject"}, func(h Header) error {
				if h.UID == 0 || h.Size == 0 || h.InternalDate.IsZero() {
					return fmt.Errorf("incomplete header %+v", h)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			err = r.FetchRaw(ctx, uids, 100<<20, func(m Message) error {
				if len(m.Raw) == 0 {
					return fmt.Errorf("%s uid %d: empty body", f.Name, m.UID)
				}
				fetched++
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		if fetched == 0 {
			t.Fatalf("%s: nothing fetched", user)
		}
		cmds := r.Commands()
		_ = r.Close()
		for _, c := range cmds {
			if !allowedCommands[c] {
				t.Errorf("%s: sent %s", user, c)
			}
		}
		if slices.Contains(cmds, "SELECT") || !slices.Contains(cmds, "EXAMINE") || !slices.Contains(cmds, "UID FETCH") {
			t.Errorf("%s: commands %v", user, cmds)
		}
		after := snapshot(t, u, user, pass)
		if !reflect.DeepEqual(before, after) {
			for name := range before {
				if !reflect.DeepEqual(before[name], after[name]) {
					t.Errorf("%s/%s changed:\n%+v\n%+v", user, name, before[name], after[name])
				}
			}
			t.Fatalf("%s: the source mailbox changed", user)
		}
	}
}
