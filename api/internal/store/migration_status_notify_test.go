package store

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStatusUpdateNotifyMigration(t *testing.T) {
	dsn := os.Getenv("YUVA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("YUVA_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	name := "yuva_mig_" + uuid.New().String()[:8]
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, `DROP DATABASE "`+name+`" WITH (FORCE)`) })

	pcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	pcfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := &Store{Pool: pool, Queries: New(pool)}
	p, err := st.migrations(slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.UpTo(ctx, 75); err != nil {
		t.Fatal(err)
	}

	ws, inbox, page := uuid.New(), uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO workspaces (id, name) VALUES ($1, 'W')`, ws)
	exec(`INSERT INTO inboxes (id, workspace_id, name, slug, identity_secret, created_at, updated_at) VALUES ($1, $2, 'I', 'i', '\x00', now(), now())`, inbox, ws)
	exec(`INSERT INTO status_pages (id, workspace_id, inbox_id, name, slug, locale, timezone, changed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'P', 'mig-page', 'en', 'UTC', now(), now(), now())`, page, ws, inbox)
	incident := func(notify bool) uuid.UUID {
		id := uuid.New()
		exec(`INSERT INTO status_incidents (id, workspace_id, page_id, kind, title, impact, state, notify, created_at, updated_at)
			VALUES ($1, $2, $3, 'incident', 'T', 'minor', 'investigating', $4, now(), now())`, id, ws, page, notify)
		return id
	}
	update := func(inc uuid.UUID, age int, notice *uuid.UUID) uuid.UUID {
		id := uuid.New()
		exec(`INSERT INTO status_incident_updates (id, workspace_id, incident_id, state, body, notice_id, created_at)
			VALUES ($1, $2, $3, 'investigating', 'b', $4, now() - make_interval(mins => $5))`, id, ws, inc, notice, age)
		return id
	}
	loud, quiet := incident(true), incident(false)
	notice := uuid.New()
	exec(`INSERT INTO status_notices (id, workspace_id, page_id, incident_id, due_at, created_at, updated_at) VALUES ($1, $2, $3, $4, now(), now(), now())`,
		notice, ws, page, loud)
	loudOpening, loudRemark, loudAttached := update(loud, 30, nil), update(loud, 20, nil), update(loud, 10, &notice)
	quietOpening, quietAttached := update(quiet, 30, nil), update(quiet, 10, &notice)

	if _, err := p.UpTo(ctx, 76); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[uuid.UUID]bool{
		loudOpening: true, loudRemark: false, loudAttached: true, quietOpening: false, quietAttached: true,
	} {
		var got bool
		if err := pool.QueryRow(ctx, `SELECT notify FROM status_incident_updates WHERE workspace_id = $1 AND id = $2`, ws, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("update %s: notify %v, want %v", id, got, want)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO status_incident_updates (id, workspace_id, incident_id, state, body, created_at) VALUES ($1, $2, $3, 'investigating', 'b', now())`,
		uuid.New(), ws, loud); err == nil {
		t.Error("an update without an explicit notify was accepted")
	}
}
