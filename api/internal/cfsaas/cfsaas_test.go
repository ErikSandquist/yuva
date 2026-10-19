package cfsaas_test

import (
	"context"
	"errors"
	"testing"

	"github.com/productdevbook/yuva/api/internal/cfsaas"
	"github.com/productdevbook/yuva/api/internal/cfsaas/cfsaastest"
)

func TestCustomHostnames(t *testing.T) {
	fake := cfsaastest.New(t, "zone1", "token1")
	c := fake.Client()
	ctx := context.Background()

	h, err := c.Create(ctx, "status.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if h.ID == "" || h.Hostname != "status.example.com" || h.Status != "pending" || h.SSL != "pending_validation" {
		t.Fatalf("created %+v", h)
	}
	if got := fake.SSLMethod("status.example.com"); got != "http" {
		t.Fatalf("ssl.method %q, want http", got)
	}

	again, err := c.Create(ctx, "status.example.com")
	if err != nil || again.ID != h.ID {
		t.Fatalf("creating it again: %+v %v", again, err)
	}

	fake.Activate("status.example.com")
	got, err := c.Get(ctx, h.ID)
	if err != nil || got.Status != "active" || got.SSL != "active" {
		t.Fatalf("after activation %+v %v", got, err)
	}
	found, err := c.Find(ctx, "status.example.com")
	if err != nil || found.ID != h.ID {
		t.Fatalf("find %+v %v", found, err)
	}

	if err := c.Delete(ctx, h.ID); err != nil {
		t.Fatal(err)
	}
	if fake.Has("status.example.com") {
		t.Fatal("still there after delete")
	}
	if err := c.Delete(ctx, h.ID); err != nil {
		t.Fatalf("deleting a deleted hostname: %v", err)
	}
	if _, err := c.Get(ctx, h.ID); !errors.Is(err, cfsaas.ErrNotFound) {
		t.Fatalf("get after delete: %v", err)
	}

	bad := fake.Client()
	bad.Token = "wrong"
	var apiErr *cfsaas.Error
	if _, err := bad.Create(ctx, "other.example.com"); !errors.As(err, &apiErr) || apiErr.Status != 403 {
		t.Fatalf("wrong token: %v", err)
	}
}
