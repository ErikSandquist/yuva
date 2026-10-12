package imapread

import (
	"bytes"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
)

// RefusedError reports a command the wire guard kept from the server. It is a bug in Yuva, never
// a problem of the mailbox.
type RefusedError struct {
	Command string
}

func (e *RefusedError) Error() string {
	return "imapread: refused to send " + e.Command + " to a source mailbox"
}

const (
	maxCommandLine = 1 << 20
	maxLiteral     = 1 << 20
)

var allowedCommands = map[string]bool{
	"CAPABILITY":   true,
	"NOOP":         true,
	"LOGOUT":       true,
	"STARTTLS":     true,
	"LOGIN":        true,
	"AUTHENTICATE": true,
	"LIST":         true,
	"STATUS":       true,
	"EXAMINE":      true,
	"UNSELECT":     true,
	"SEARCH":       true,
	"UID SEARCH":   true,
	"FETCH":        true,
	"UID FETCH":    true,
}

var allowedMechanisms = map[string]bool{"PLAIN": true, "XOAUTH2": true}

var allowedFetchAtoms = map[string]bool{
	"UID": true, "FLAGS": true, "INTERNALDATE": true, "RFC822.SIZE": true, "ENVELOPE": true,
	"BODYSTRUCTURE": true, "MODSEQ": true,
}

// guard sits under the IMAP client and parses every command it writes. A command outside the
// allowlist closes the connection before any of its bytes leave the process.
type guard struct {
	net.Conn

	mu          sync.Mutex
	line        []byte
	literal     int64
	inCommand   bool
	authPending bool
	refused     error
	commands    []string
}

func newGuard(conn net.Conn) *guard { return &guard{Conn: conn} }

func (g *guard) Commands() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.commands...)
}

func (g *guard) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.refused != nil {
		return 0, g.refused
	}
	var out []byte
	for i := 0; i < len(p); {
		if g.literal > 0 {
			n := min(int64(len(p)-i), g.literal)
			out = append(out, p[i:i+int(n)]...)
			g.literal -= n
			i += int(n)
			continue
		}
		j := bytes.IndexByte(p[i:], '\n')
		if j < 0 {
			g.line = append(g.line, p[i:]...)
			if len(g.line) > maxCommandLine {
				return 0, g.refuse("an over-long command")
			}
			break
		}
		g.line = append(g.line, p[i:i+j+1]...)
		i += j + 1
		seg, err := g.segment()
		if err != nil {
			return 0, err
		}
		out = append(out, seg...)
	}
	if len(out) > 0 {
		if _, err := g.Conn.Write(out); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// segment checks one CRLF-terminated piece of the stream: a whole command, the part of one up to
// a literal, the rest after a literal, or a SASL response line.
func (g *guard) segment() ([]byte, error) {
	seg := g.line
	g.line = nil
	if !bytes.HasSuffix(seg, []byte("\r\n")) {
		return nil, g.refuse("a line not ending in CRLF")
	}
	body := string(seg[:len(seg)-2])
	size, hasLiteral, err := trailingLiteral(body)
	if err != nil {
		return nil, g.refuse("a malformed literal")
	}
	if hasLiteral {
		g.literal = size
	}
	if g.inCommand {
		g.inCommand = hasLiteral
		return seg, nil
	}
	if g.authPending && !hasLiteral && saslLine(body) {
		return seg, nil
	}
	g.authPending = false
	name, err := checkCommand(body, hasLiteral)
	if err != nil {
		var refused *RefusedError
		if errors.As(err, &refused) {
			return nil, g.refuse(refused.Command)
		}
		return nil, err
	}
	g.commands = append(g.commands, name)
	g.inCommand = hasLiteral
	g.authPending = name == "AUTHENTICATE"
	return seg, nil
}

func (g *guard) refuse(what string) error {
	g.refused = &RefusedError{Command: what}
	g.line = nil
	_ = g.Conn.Close()
	return g.refused
}

func trailingLiteral(s string) (int64, bool, error) {
	if !strings.HasSuffix(s, "}") {
		return 0, false, nil
	}
	open := strings.LastIndexByte(s, '{')
	if open < 0 {
		return 0, false, nil
	}
	digits := strings.TrimSuffix(s[open+1:len(s)-1], "+")
	if digits == "" {
		return 0, false, errors.New("empty literal")
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || n < 0 || n > maxLiteral {
		return 0, false, errors.New("bad literal")
	}
	return n, true, nil
}

func saslLine(s string) bool {
	if s == "*" {
		return true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '=') {
			return false
		}
	}
	return true
}

// checkCommand reads the tag and the command name from the first piece of a command and answers
// its normalised name, or a RefusedError.
func checkCommand(s string, hasLiteral bool) (string, error) {
	tag, rest, ok := strings.Cut(s, " ")
	if !ok || !validTag(tag) {
		return "", &RefusedError{Command: "a line that is not a command"}
	}
	name, args, _ := strings.Cut(rest, " ")
	name = strings.ToUpper(name)
	if name == "UID" {
		sub, subArgs, _ := strings.Cut(args, " ")
		name, args = "UID "+strings.ToUpper(sub), subArgs
	}
	if !allowedCommands[name] {
		return "", &RefusedError{Command: printable(name)}
	}
	switch name {
	case "AUTHENTICATE":
		mech, _, _ := strings.Cut(args, " ")
		if !allowedMechanisms[strings.ToUpper(mech)] {
			return "", &RefusedError{Command: "AUTHENTICATE " + printable(strings.ToUpper(mech))}
		}
	case "FETCH", "UID FETCH":
		if hasLiteral {
			return "", &RefusedError{Command: name + " with a literal"}
		}
		if item, ok := checkFetch(args); !ok {
			return "", &RefusedError{Command: name + " " + printable(item)}
		}
	case "STARTTLS", "CAPABILITY", "NOOP", "LOGOUT", "UNSELECT":
		if strings.TrimSpace(args) != "" {
			return "", &RefusedError{Command: name + " with arguments"}
		}
	}
	return name, nil
}

func validTag(t string) bool {
	if t == "" || t == "+" || t == "*" {
		return false
	}
	for i := 0; i < len(t); i++ {
		c := t[i]
		if c <= ' ' || c >= 0x7f || strings.IndexByte(`(){%*"\+`, c) >= 0 {
			return false
		}
	}
	return true
}

func printable(s string) string {
	if len(s) > 40 {
		s = s[:40]
	}
	return strings.Map(func(r rune) rune {
		if r < ' ' || r > '~' {
			return '?'
		}
		return r
	}, s)
}

// checkFetch accepts `<set> <items> [(CHANGEDSINCE n)]` whose items mark nothing as seen, and
// otherwise answers the first item it refused.
func checkFetch(args string) (string, bool) {
	set, rest, ok := strings.Cut(args, " ")
	if !ok || !validSet(set) {
		return "sequence set", false
	}
	rest = strings.TrimSpace(rest)
	var items []string
	if strings.HasPrefix(rest, "(") {
		end := closing(rest, 0)
		if end < 0 {
			return "item list", false
		}
		list := rest[1:end]
		rest = strings.TrimSpace(rest[end+1:])
		var ok bool
		if items, ok = splitItems(list); !ok {
			return "item list", false
		}
	} else {
		item, after, _ := cutItem(rest)
		items, rest = []string{item}, strings.TrimSpace(after)
	}
	if len(items) == 0 {
		return "item list", false
	}
	for _, it := range items {
		if !fetchItemAllowed(it) {
			return it, false
		}
	}
	if rest != "" && !changedSince(rest) {
		return rest, false
	}
	return "", true
}

func validSet(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c == ',' || c == ':' || c == '*' || c == '$') {
			return false
		}
	}
	return true
}

func changedSince(s string) bool {
	if !strings.HasPrefix(s, "(") || !strings.HasSuffix(s, ")") {
		return false
	}
	f := strings.Fields(s[1 : len(s)-1])
	if len(f) != 2 || !strings.EqualFold(f[0], "CHANGEDSINCE") {
		return false
	}
	_, err := strconv.ParseUint(f[1], 10, 64)
	return err == nil
}

// closing finds the parenthesis that closes s[open], skipping quoted strings and sections.
func closing(s string, open int) int {
	depth := 0
	inQuote := false
	for i := open; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote && c == '\\':
			i++
		case c == '"':
			inQuote = !inQuote
		case inQuote:
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func cutItem(s string) (string, string, bool) {
	depth := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote && c == '\\':
			i++
		case c == '"':
			inQuote = !inQuote
		case inQuote:
		case c == '[' || c == '(' || c == '<':
			depth++
		case c == ']' || c == ')' || c == '>':
			depth--
		case c == ' ' && depth == 0:
			return s[:i], s[i+1:], true
		}
	}
	return s, "", depth == 0 && !inQuote
}

func splitItems(s string) ([]string, bool) {
	var out []string
	s = strings.TrimSpace(s)
	for s != "" {
		item, rest, ok := cutItem(s)
		if !ok {
			return nil, false
		}
		out = append(out, item)
		s = strings.TrimSpace(rest)
	}
	return out, true
}

func fetchItemAllowed(item string) bool {
	upper := strings.ToUpper(item)
	if allowedFetchAtoms[upper] {
		return true
	}
	if !strings.HasPrefix(upper, "BODY.PEEK[") {
		return false
	}
	end := strings.LastIndexByte(upper, ']')
	if end < 0 {
		return false
	}
	partial := upper[end+1:]
	if partial == "" {
		return true
	}
	if !strings.HasPrefix(partial, "<") || !strings.HasSuffix(partial, ">") {
		return false
	}
	for _, c := range partial[1 : len(partial)-1] {
		if !(c >= '0' && c <= '9' || c == '.') {
			return false
		}
	}
	return true
}
