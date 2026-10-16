package api_test

import (
	"net/http"
	"testing"
)

func TestConsentHistoryIsForManagersAndKeys(t *testing.T) {
	h := newHarness(t)
	lt := newListTeam(t, h)
	sub := lt.owner.expect(http.StatusCreated, "POST", "/v1/lists/"+lt.list+"/subscriptions", map[string]any{
		"email": "reader@example.org", "consent": map[string]any{"confirmed": true, "ip": "203.0.113.9", "text": "Ticked the box"},
	})
	path := "/v1/lists/" + lt.list + "/subscriptions/" + sub.str("id")
	contact := sub.body["contact"].(map[string]any)["id"].(string)

	got := lt.owner.expect(http.StatusOK, "GET", path, nil)
	if len(got.body["events"].([]any)) == 0 {
		t.Fatalf("owner sees no events: %s", got.raw)
	}
	lt.owner.expect(http.StatusOK, "GET", "/v1/lists/"+lt.list+"/consent-export?format=json", nil)
	lt.owner.expect(http.StatusOK, "GET", "/v1/contacts/"+contact+"/consent?format=json", nil)

	lt.agent.expect(http.StatusOK, "GET", "/v1/lists/"+lt.list+"/subscriptions", nil)
	lt.agent.expectProblem(http.StatusForbidden, "forbidden", "GET", path, nil)
	lt.agent.expectProblem(http.StatusForbidden, "forbidden", "GET", "/v1/lists/"+lt.list+"/consent-export?format=json", nil)
	lt.agent.expectProblem(http.StatusForbidden, "forbidden", "GET", "/v1/contacts/"+contact+"/consent?format=json", nil)

	reader := h.client()
	reader.bearer = lt.owner.expect(http.StatusCreated, "POST", "/v1/api-keys", map[string]any{"name": "reader", "scopes": []string{"lists:read", "contacts:read"}}).str("secret")
	reader.expect(http.StatusOK, "GET", path, nil)

	foreign := newTeam(t, h)
	foreign.owner.expectProblem(http.StatusNotFound, "not_found", "GET", path, nil)
	foreign.owner.expectProblem(http.StatusNotFound, "not_found", "GET", "/v1/contacts/"+contact+"/consent", nil)
}
