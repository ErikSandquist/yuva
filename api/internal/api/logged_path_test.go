package api

import "testing"

func TestLoggedPathMasksTokenPrefixes(t *testing.T) {
	for _, prefix := range tokenPathPrefixes {
		if got, want := loggedPath(prefix+"secret"), prefix+"…"; got != want {
			t.Errorf("%s: got %q, want %q", prefix, got, want)
		}
		if got, want := loggedPath(prefix+"secret/data"), prefix+"…/data"; got != want {
			t.Errorf("%s with tail: got %q, want %q", prefix, got, want)
		}
	}
	for _, p := range []string{"/v1/conversations", "/client/v1/messages", "/"} {
		if got := loggedPath(p); got != p {
			t.Errorf("%s changed to %q", p, got)
		}
	}
}
