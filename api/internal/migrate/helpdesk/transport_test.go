package helpdesk

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// noNetwork fails a dial, and counts it: a refused request must never reach one.
func noNetwork(dials *atomic.Int32) Config {
	return Config{Dial: func(context.Context, string, string) (net.Conn, error) {
		dials.Add(1)
		return nil, errors.New("no network in this test")
	}}
}

func refused(t *testing.T, c *http.Client, method, url string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Do(req)
	if res != nil {
		res.Body.Close()
	}
	var r *RefusedError
	if !errors.As(err, &r) {
		t.Errorf("%s %s: %v, want a refusal", method, url, err)
	}
}

func hasGuard(rt http.RoundTripper) bool {
	for {
		switch v := rt.(type) {
		case getOnly:
			return true
		case bearer:
			rt = v.next
		default:
			return false
		}
	}
}

// A help desk's data clients send only GETs, over HTTPS, to their own hosts, and the token client
// only a POST to the token endpoint: anything else is refused before a connection is made. This
// fails the build if a reader could send a request that changes the help desk.
func TestSourceTransportsGETOnly(t *testing.T) {
	var dials atomic.Int32
	cfg := noNetwork(&dials)
	token := func() string { return "secret-token" }
	z := NewZendesk(cfg, "acme", token, nil)
	h := NewHelpScout(cfg, token, nil)
	clients := map[string]*http.Client{"zendesk api": z.c.api, "zendesk files": z.c.files, "helpscout api": h.c.api, "helpscout files": h.c.files}
	for name, c := range clients {
		if !hasGuard(c.Transport) {
			t.Errorf("%s is not built on the GET-only transport", name)
		}
	}
	hosts := map[string]string{"zendesk api": "https://acme.zendesk.com", "zendesk files": "https://acme.zendesk.com",
		"helpscout api": "https://api.helpscout.net", "helpscout files": "https://api.helpscout.net"}
	for name, c := range clients {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, "PROPFIND"} {
			refused(t, c, method, hosts[name]+"/api/v2/tickets/1")
		}
		refused(t, c, http.MethodGet, strings.Replace(hosts[name], "https://", "http://", 1)+"/api/v2/tickets/1")
	}
	for _, c := range []*http.Client{z.c.api, h.c.api} {
		refused(t, c, http.MethodGet, "https://evil.example.com/steal")
	}
	refused(t, h.c.files, http.MethodGet, "https://files.example.org/a.txt")
	for _, tc := range []struct{ url, method, target string }{
		{ZendeskToken("acme"), http.MethodGet, ZendeskToken("acme")},
		{ZendeskToken("acme"), http.MethodPost, "https://acme.zendesk.com/api/v2/tickets/1"},
		{ZendeskToken("acme"), http.MethodDelete, "https://acme.zendesk.com/api/v2/oauth/tokens/current"},
		{HelpScoutToken, http.MethodPut, HelpScoutToken},
		{HelpScoutToken, http.MethodPost, "https://api.helpscout.net/v2/conversations"},
	} {
		refused(t, cfg.tokenClient(tc.url), tc.method, tc.target)
	}
	if n := dials.Load(); n != 0 {
		t.Errorf("%d connections were opened for refused requests", n)
	}
}

// The token goes only to the account's own hosts.
func TestBearerOnlyToTokenHosts(t *testing.T) {
	var seen []string
	rt := bearer{hosts: Hosts{Exact: []string{"acme.zendesk.com"}, Suffixes: []string{"zdusercontent.com"}}, token: func() string { return "tok" },
		next: roundTrip(func(r *http.Request) (*http.Response, error) {
			seen = append(seen, r.URL.Host+"="+r.Header.Get("Authorization"))
			return nil, errors.New("stop")
		})}
	for _, u := range []string{"https://acme.zendesk.com/a", "https://p1.zdusercontent.com/b", "https://files.example.org/c", "https://zdusercontent.com.evil.example/d"} {
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		req.Header.Set("Authorization", "Bearer leaked")
		_, _ = rt.RoundTrip(req)
	}
	want := []string{"acme.zendesk.com=Bearer tok", "p1.zdusercontent.com=Bearer tok", "files.example.org=", "zdusercontent.com.evil.example="}
	if strings.Join(seen, " ") != strings.Join(want, " ") {
		t.Errorf("sent %v, want %v", seen, want)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Every request this package builds is a GET, except the one POST in token to the token endpoint;
// no other way to send a request is used. This fails the build when another is added.
func TestOnlyGETRequestsInSource(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := gotoken.NewFileSet()
	newRequests := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "Post", "PostForm", "Head", "Put", "Patch", "Delete":
					t.Errorf("%s: %s calls %s", fset.Position(call.Pos()), fn.Name.Name, sel.Sel.Name)
				case "NewRequest", "NewRequestWithContext":
					newRequests++
					arg := call.Args[0]
					if sel.Sel.Name == "NewRequestWithContext" {
						arg = call.Args[1]
					}
					m, ok := arg.(*ast.SelectorExpr)
					want := "MethodGet"
					if fn.Name.Name == "token" {
						want = "MethodPost"
					}
					if !ok || m.Sel.Name != want {
						t.Errorf("%s: %s builds a request whose method is not http.%s", fset.Position(call.Pos()), fn.Name.Name, want)
					}
				}
				return true
			})
		}
	}
	if newRequests != 2 {
		t.Errorf("%d places build requests, want the reader's GET and the token exchange's POST", newRequests)
	}
}
