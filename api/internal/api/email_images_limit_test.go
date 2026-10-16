package api_test

import (
	"image/color"
	"net/http"
	"testing"
)

func TestEmailImageUploadsAreRateLimitedPerWorkspace(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	other := newTeam(t, h)
	pic := testPNG(t, 8, 8, color.RGBA{30, 64, 175, 255})

	for i := range 30 {
		if r := tm.owner.uploadImage("a.png", "image/png", pic); r.status != http.StatusCreated && r.status != http.StatusOK {
			t.Fatalf("upload %d: %d %s", i, r.status, r.raw)
		}
	}
	r := tm.owner.uploadImage("a.png", "image/png", pic)
	if r.status != http.StatusTooManyRequests || r.str("code") != "rate_limited" || r.header.Get("Retry-After") == "" {
		t.Fatalf("31st upload: %d %s", r.status, r.raw)
	}
	if r := other.owner.uploadImage("a.png", "image/png", pic); r.status != http.StatusCreated {
		t.Fatalf("another workspace: %d %s", r.status, r.raw)
	}
}
