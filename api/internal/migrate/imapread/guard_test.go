package imapread

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"testing"
)

type recordConn struct {
	net.Conn
	written bytes.Buffer
	closed  bool
}

func (c *recordConn) Write(p []byte) (int, error) { return c.written.Write(p) }
func (c *recordConn) Close() error                { c.closed = true; return nil }

var refusedCommands = []string{
	`SELECT INBOX`,
	`STORE 1 +FLAGS (\Seen)`,
	`STORE 1:* +FLAGS.SILENT (\Deleted)`,
	`COPY 1 Archive`,
	`MOVE 1 Trash`,
	`EXPUNGE`,
	`APPEND INBOX {3}`,
	`CREATE Yuva`,
	`DELETE INBOX`,
	`RENAME INBOX Old`,
	`SUBSCRIBE Sent`,
	`UNSUBSCRIBE Sent`,
	`CLOSE`,
	`IDLE`,
	`ENABLE CONDSTORE`,
	`SETACL INBOX anyone lrswipkxtecda`,
	`DELETEACL INBOX anyone`,
	`SETMETADATA INBOX (/private/comment "x")`,
	`SETQUOTA "" (STORAGE 1)`,
	`ID NIL`,
	`NAMESPACE`,
	`COMPRESS DEFLATE`,
	`XLIST "" "*"`,
	`UNAUTHENTICATE`,
	`AUTHENTICATE LOGIN`,
	`AUTHENTICATE CRAM-MD5`,
	`AUTHENTICATE GSSAPI`,
	`FETCH 1 BODY[]`,
	`FETCH 1 (BODY[TEXT])`,
	`FETCH 1 (UID BODY[HEADER.FIELDS (SUBJECT)])`,
	`FETCH 1 RFC822`,
	`FETCH 1 (RFC822.TEXT)`,
	`FETCH 1 (RFC822.HEADER)`,
	`FETCH 1 (BINARY[1])`,
	`FETCH 1 (BINARY.PEEK[1])`,
	`FETCH 1 FULL`,
	`FETCH 1 ALL`,
	`FETCH 1 (UID FLAGS) (VANISHED)`,
	`FETCH 1 (UID BODY.PEEK[]<0.x>)`,
	`FETCH 1:* (UID BODY.PEEK[] BODY[])`,
	`STARTTLS extra`,
	`NOOP STORE`,
}

var uidRefused = []string{`STORE 1 +FLAGS (\Seen)`, `COPY 1 Archive`, `MOVE 1 Trash`, `EXPUNGE 1`, `FETCH 1 (BODY[])`, `FETCH 1 RFC822`, `THREAD REFERENCES UTF-8 ALL`}

var allowed = []string{
	"a1 CAPABILITY\r\n",
	"a2 NOOP\r\n",
	"a3 STARTTLS\r\n",
	"a4 LOGIN \"user@example.com\" \"secret\"\r\n",
	"a5 LOGIN {16}\r\nuser@example.com {6}\r\nsecret\r\n",
	"a6 AUTHENTICATE PLAIN\r\nAHVzZXJAZXhhbXBsZS5jb20Ac2VjcmV0\r\n",
	"a7 AUTHENTICATE PLAIN AHVzZXJAZXhhbXBsZS5jb20Ac2VjcmV0\r\n",
	"a8 AUTHENTICATE XOAUTH2 dXNlcj1zb21ldXNlckBleGFtcGxlLmNvbQFhdXRoPUJlYXJlciB2RjlkZnQ0cWZUYzJOdmIzUmxia0JoZEhSaGRtbHpkR0V1WTI5dENnPT0BAQ==\r\n\r\n",
	"a9 LIST \"\" \"*\" RETURN (SPECIAL-USE)\r\n",
	"b1 LIST \"\" \"*\"\r\n",
	"b2 STATUS INBOX (MESSAGES UIDNEXT UIDVALIDITY HIGHESTMODSEQ)\r\n",
	"b3 EXAMINE INBOX\r\n",
	"b4 EXAMINE \"[Gmail]/All Mail\" (CONDSTORE)\r\n",
	"b5 UNSELECT\r\n",
	"b6 SEARCH ALL\r\n",
	"b7 UID SEARCH SINCE 1-Jan-2024\r\n",
	"b8 UID SEARCH CHARSET UTF-8 SUBJECT {4}\r\nçok\r\n",
	"b9 UID FETCH 1:* (UID INTERNALDATE RFC822.SIZE FLAGS BODY.PEEK[HEADER.FIELDS (\"Message-ID\" \"In-Reply-To\" \"References\")])\r\n",
	"c1 UID FETCH 5,7:9 (UID BODY.PEEK[])\r\n",
	"c2 FETCH 1 (UID ENVELOPE BODYSTRUCTURE MODSEQ)\r\n",
	"c3 UID FETCH 1:100 (UID FLAGS) (CHANGEDSINCE 12345)\r\n",
	"c4 uid fetch 1 (uid body.peek[1.2]<0.1024>)\r\n",
	"c5 FETCH 1 UID\r\n",
	"c6 UID FETCH 1 BODY.PEEK[TEXT]\r\n",
	"c7 LOGOUT\r\n",
}

func TestIMAPGuardRefusesWrites(t *testing.T) {
	var cases []string
	for _, c := range refusedCommands {
		cases = append(cases, "x1 "+c+"\r\n", "x1 "+strings.ToLower(c)+"\r\n")
	}
	for _, c := range uidRefused {
		cases = append(cases, "x2 UID "+c+"\r\n", "x2 uid "+strings.ToLower(c)+"\r\n")
	}
	cases = append(cases,
		"x3 STORE\r\n",
		"x4\r\n",
		"* STORE 1 +FLAGS (\\Seen)\r\n",
		"+ STORE 1 +FLAGS (\\Seen)\r\n",
		"x5 UID\r\n",
		"x6 FETCH 1 (BODY.PEEK[HEADER.FIELDS {7}\r\n",
		"x7 STORE 1 +FLAGS {6}\r\n",
		"x8 STORE 1 +FLAGS (\\Seen)\n",
	)
	for _, c := range cases {
		for _, split := range []bool{false, true} {
			conn := &recordConn{}
			g := newGuard(conn)
			err := write(g, c, split)
			var refused *RefusedError
			if !errors.As(err, &refused) {
				t.Errorf("%q (split %v): not refused, err %v", c, split, err)
				continue
			}
			if conn.written.Len() != 0 {
				t.Errorf("%q (split %v): %q reached the server", c, split, conn.written.String())
			}
			if !conn.closed {
				t.Errorf("%q (split %v): connection left open", c, split)
			}
			if _, err := g.Write([]byte("a1 NOOP\r\n")); err == nil || conn.written.Len() != 0 {
				t.Errorf("%q (split %v): the guard wrote after refusing", c, split)
			}
		}
	}
}

func TestIMAPGuardRefusesAfterAllowed(t *testing.T) {
	for _, c := range []string{
		"a1 LOGIN {4}\r\nuser {5}\r\nx\r\n\r\n\r\na2 STORE 1 +FLAGS (\\Seen)\r\n",
		"a1 UID SEARCH SUBJECT {20}\r\na2 STORE 1 +FLAGS ()\r\n\r\na3 EXPUNGE\r\n",
		"a1 AUTHENTICATE PLAIN\r\nAAAA\r\na2 DELETE INBOX\r\n",
		"a1 NOOP\r\nYTEgU1RPUkU=\r\n",
	} {
		for _, split := range []bool{false, true} {
			conn := &recordConn{}
			g := newGuard(conn)
			err := write(g, c, split)
			var refused *RefusedError
			if !errors.As(err, &refused) {
				t.Errorf("%q (split %v): not refused, err %v", c, split, err)
				continue
			}
			if strings.Contains(conn.written.String(), "STORE 1") && !strings.Contains(c, "{20}") {
				t.Errorf("%q (split %v): refused command reached the server: %q", c, split, conn.written.String())
			}
			if strings.Contains(conn.written.String(), "EXPUNGE") || strings.Contains(conn.written.String(), "DELETE") {
				t.Errorf("%q (split %v): refused command reached the server: %q", c, split, conn.written.String())
			}
		}
	}
}

func TestIMAPGuardPassesReads(t *testing.T) {
	for _, split := range []bool{false, true} {
		conn := &recordConn{}
		g := newGuard(conn)
		var all string
		for _, c := range allowed {
			if err := write(g, c, split); err != nil {
				t.Fatalf("%q (split %v): %v", c, split, err)
			}
			all += c
		}
		if conn.written.String() != all {
			t.Fatalf("split %v: bytes changed on the way:\n%q\n%q", split, conn.written.String(), all)
		}
		if conn.closed {
			t.Fatalf("split %v: connection closed", split)
		}
		want := []string{"CAPABILITY", "NOOP", "STARTTLS", "LOGIN", "LOGIN", "AUTHENTICATE", "AUTHENTICATE", "AUTHENTICATE",
			"LIST", "LIST", "STATUS", "EXAMINE", "EXAMINE", "UNSELECT", "SEARCH", "UID SEARCH", "UID SEARCH", "UID FETCH",
			"UID FETCH", "FETCH", "UID FETCH", "UID FETCH", "FETCH", "UID FETCH", "LOGOUT"}
		if got := g.Commands(); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("split %v: commands %v", split, got)
		}
	}
}

func TestIMAPGuardLiteralData(t *testing.T) {
	conn := &recordConn{}
	g := newGuard(conn)
	in := "a1 UID SEARCH SUBJECT {26}\r\nx STORE 1 +FLAGS (\\Seen)\r\n\r\n"
	if err := write(g, in, true); err != nil {
		t.Fatal(err)
	}
	if conn.written.String() != in {
		t.Fatalf("literal data changed: %q", conn.written.String())
	}
}

func write(g *guard, s string, split bool) error {
	if !split {
		_, err := g.Write([]byte(s))
		return err
	}
	for i := 0; i < len(s); i++ {
		if _, err := g.Write([]byte{s[i]}); err != nil {
			return err
		}
	}
	return nil
}
