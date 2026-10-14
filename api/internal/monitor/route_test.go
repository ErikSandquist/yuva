package monitor

import "testing"

func TestInferRoute(t *testing.T) {
	for _, c := range []struct{ path, want string }{
		{"/users/42", "/users/{id}"},
		{"/users/42/orders/7?expand=items#top", "/users/{id}/orders/{id}"},
		{"/v1/accounts/0b6a3e9a-2c55-4f7e-9c4f-58f1a5d2a0b1", "/v1/accounts/{id}"},
		{"/v1/accounts/0b6a3e9a2c554f7e9c4f58f1a5d2a0b1/keys", "/v1/accounts/{id}/keys"},
		{"/reports/2026-10-12", "/reports/{id}"},
		{"/blobs/deadbeefcafebabe1234", "/blobs/{id}"},
		{"/tokens/01ARZ3NDEKTSV4RRFFQ69G5FAV", "/tokens/{id}"},
		{"/customers/cus_NffrFeUfNV2Hib", "/customers/{id}"},
		{"/people/ayse@example.com", "/people/{id}"},
		{"/people/ayse%40example.com/notes", "/people/{id}/notes"},
		{"/search/" + "abcdefghijklmnopqrstuvwxyzabcdefghijklmnop", "/search/{id}"},
		{"/user-preferences/notification-settings", "/user-preferences/notification-settings"},
		{"/oauth2-callback-handler", "/oauth2-callback-handler"},
		{"/v2/items", "/v2/items"},
		{"/users/{id}", "/users/{id}"},
		{"/users/:id/posts/*", "/users/:id/posts/*"},
		{"/", "/"},
		{"", "/"},
		{"users/42", "/users/{id}"},
	} {
		if got := InferRoute(c.path); got != c.want {
			t.Errorf("InferRoute(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestMethodsAndStatus(t *testing.T) {
	for _, c := range []struct {
		in   string
		want string
	}{{"get", "GET"}, {"PATCH", "PATCH"}, {"PROPFIND", "_OTHER"}, {"", "_OTHER"}, {"QUERY", "_OTHER"}} {
		if got := MethodName(Method(c.in)); got != c.want {
			t.Errorf("method %q = %q, want %q", c.in, got, c.want)
		}
	}
	if !IsError(0) || !IsError(503) || IsError(404) || !IsClientError(404) || IsClientError(500) || IsClientError(399) {
		t.Fatal("error classes")
	}
	if StatusClassName(StatusClass(0)) != "none" || StatusClassName(StatusClass(204)) != "2xx" || StatusClassName(StatusClass(599)) != "5xx" {
		t.Fatal("status classes")
	}
}
