package api_test

import (
	"net/http"
	"testing"

	"github.com/productdevbook/yuva/api/internal/api"
)

func openSignup(perPerson, perIP int) func(*api.Deps) {
	return func(d *api.Deps) {
		d.Auth.Signup = api.SignupSettings{Open: true, WorkspacesPerPerson: perPerson, WorkspacesPerIP: perIP}
	}
}

func TestSignupInviteOnlyByDefault(t *testing.T) {
	h := newHarness(t)
	c := h.client()
	if v := c.expect(http.StatusOK, "GET", "/v1/version", nil); v.str("signup") != "invite" {
		t.Fatalf("version: %s", v.raw)
	}
	stranger := unique("stranger") + "@example.com"
	c.expect(http.StatusAccepted, "POST", "/v1/auth/code", map[string]any{"email": stranger})
	if n := len(h.mail.to(stranger)); n != 0 {
		t.Fatalf("%d mails sent to an unknown address", n)
	}

	email := unique("owner") + "@example.com"
	h.bootstrap(email, unique("ws"))
	c.signIn(email)
	c.expectProblem(http.StatusForbidden, "signup_closed", "POST", "/v1/workspaces", map[string]any{"name": "Acme"})
}

func TestSignupOpen(t *testing.T) {
	h := newHarnessWith(t, openSignup(2, 10))
	other := unique("other") + "@example.com"
	theirs := h.bootstrap(other, unique("ws-other"))

	c := h.client()
	if v := c.expect(http.StatusOK, "GET", "/v1/version", nil); v.str("signup") != "open" {
		t.Fatalf("version: %s", v.raw)
	}
	c.expectProblem(http.StatusUnauthorized, "unauthenticated", "POST", "/v1/workspaces", map[string]any{"name": "Acme"})

	email := unique("new") + "@example.com"
	c.signIn(email)
	me := c.expect(http.StatusOK, "GET", "/v1/me", nil)
	if ms := me.body["memberships"].([]any); len(ms) != 0 {
		t.Fatalf("a new person has memberships: %s", me.raw)
	}

	c.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/workspaces", map[string]any{"name": "   "})
	c.workspace = theirs.WorkspaceID.String()
	ws := c.expect(http.StatusCreated, "POST", "/v1/workspaces", map[string]any{"name": "  Acme  "})
	if ws.str("name") != "Acme" || ws.str("id") == theirs.WorkspaceID.String() {
		t.Fatalf("workspace: %s", ws.raw)
	}

	me = c.expect(http.StatusOK, "GET", "/v1/me", nil)
	ms := me.body["memberships"].([]any)
	if len(ms) != 1 || ms[0].(map[string]any)["workspace"].(map[string]any)["id"] != ws.str("id") || ms[0].(map[string]any)["role"] != "owner" {
		t.Fatalf("memberships: %s", me.raw)
	}
	c.expectProblem(http.StatusForbidden, "not_a_member", "GET", "/v1/members", nil)

	c.workspace = ws.str("id")
	if got := c.expect(http.StatusOK, "GET", "/v1/workspace", nil); got.str("id") != ws.str("id") {
		t.Fatalf("workspace: %s", got.raw)
	}
	members := c.expect(http.StatusOK, "GET", "/v1/members", nil)
	if items := members.body["items"].([]any); len(items) != 1 || items[0].(map[string]any)["email"] != email {
		t.Fatalf("members: %s", members.raw)
	}
	if inboxes := c.expect(http.StatusOK, "GET", "/v1/inboxes", nil); len(inboxes.body["items"].([]any)) != 0 {
		t.Fatalf("inboxes: %s", inboxes.raw)
	}
	c.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/members/"+theirs.MemberID.String(), nil)

	c.expect(http.StatusCreated, "POST", "/v1/workspaces", map[string]any{"name": "Second"})
	c.expectProblem(http.StatusForbidden, "workspace_limit", "POST", "/v1/workspaces", map[string]any{"name": "Third"})
}

func TestSignupWorkspacesPerIP(t *testing.T) {
	h := newHarnessWith(t, openSignup(5, 2))
	ip := h.client().ip
	for i := range 3 {
		c := h.client()
		c.ip = ip
		c.signIn(unique("new") + "@example.com")
		if i < 2 {
			c.expect(http.StatusCreated, "POST", "/v1/workspaces", map[string]any{"name": "Acme"})
		} else {
			c.expectProblem(http.StatusTooManyRequests, "rate_limited", "POST", "/v1/workspaces", map[string]any{"name": "Acme"})
		}
	}
}
