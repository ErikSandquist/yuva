package analytics

import (
	"context"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
)

// MaxVisitPages is how many of a visit's paths a contact's visit lists.
const MaxVisitPages = 20

// ContactVisit is a visit linked to a contact, described as its first event saw it.
type ContactVisit struct {
	PropertyID                                       uuid.UUID
	PropertyName                                     string
	Day                                              time.Time
	Visit                                            int64
	Started, Ended                                   time.Time
	Hostname, Entry, Exit                            string
	Views, Events                                    int32
	Channel, Source, Campaign, Country, City, Device string
	Browser, OS, Platform, AppVersion, VisitorType   string
	Pages                                            []VisitPage
}

type VisitPage struct {
	Path  string
	Views int32
}

// VisitCursor is the last visit of a page of a contact's visits.
type VisitCursor struct {
	Started time.Time
	Visit   int64
}

// ContactVisits lists a contact's visits that are not a bot's, newest first, on the properties of
// the inboxes given (on every property when inboxes is nil), each with its most viewed paths.
func (s *PGStore) ContactVisits(ctx context.Context, ws, contactID uuid.UUID, inboxes []uuid.UUID, after *VisitCursor, limit int) ([]ContactVisit, error) {
	dim := func(name string) string {
		d, _ := DimByName(name)
		return d.Group
	}
	var at *time.Time
	var visit int64
	if after != nil {
		at, visit = &after.Started, after.Visit
	}
	rows, err := s.Pool.Query(ctx, `SELECT v.property_id, p.name, v.day, v.visit, v.started_at, v.ended_at, v.hostname, v.entry_path, v.exit_path,
	v.views, v.events, `+dim("channel")+`, v.source, v.utm_campaign, v.country, v.city, `+dim("device")+`, v.browser, v.os,
	`+dim("platform")+`, v.app_version, `+dim("visitor_type")+`
FROM analytics_visits v
JOIN analytics_properties p ON p.workspace_id = v.workspace_id AND p.id = v.property_id AND p.deleted_at IS NULL
WHERE v.workspace_id = $1 AND v.contact_id = $2 AND NOT v.bot AND ($3::boolean OR p.inbox_id = ANY($4::uuid[]))
    AND ($5::timestamptz IS NULL OR (v.started_at, v.visit) < ($5, $6))
ORDER BY v.started_at DESC, v.visit DESC
LIMIT $7`, ws, contactID, inboxes == nil, inboxes, at, visit, limit)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (ContactVisit, error) {
		var c ContactVisit
		err := r.Scan(&c.PropertyID, &c.PropertyName, &c.Day, &c.Visit, &c.Started, &c.Ended, &c.Hostname, &c.Entry, &c.Exit, &c.Views, &c.Events,
			&c.Channel, &c.Source, &c.Campaign, &c.Country, &c.City, &c.Device, &c.Browser, &c.OS, &c.Platform, &c.AppVersion, &c.VisitorType)
		c.Day, c.Started, c.Ended = c.Day.UTC(), c.Started.UTC(), c.Ended.UTC()
		return c, err
	})
	if err != nil || len(out) == 0 {
		return out, err
	}
	props, days, visits := make([]uuid.UUID, len(out)), make([]time.Time, len(out)), make([]int64, len(out))
	index := map[visitKey3]int{}
	for i, c := range out {
		props[i], days[i], visits[i] = c.PropertyID, c.Day, c.Visit
		index[visitKey3{c.PropertyID, c.Day, c.Visit}] = i
	}
	rows, err = s.Pool.Query(ctx, `SELECT property_id, day, visit, path, views FROM analytics_visit_pages
WHERE workspace_id = $1 AND (property_id, day, visit) IN (SELECT * FROM unnest($2::uuid[], $3::date[], $4::bigint[])) AND views > 0
ORDER BY views DESC, path`, ws, props, days, visits)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k visitKey3
		var pg VisitPage
		if err := rows.Scan(&k.property, &k.day, &k.visit, &pg.Path, &pg.Views); err != nil {
			return nil, err
		}
		k.day = k.day.UTC()
		if i, ok := index[k]; ok && len(out[i].Pages) < MaxVisitPages {
			out[i].Pages = append(out[i].Pages, pg)
		}
	}
	return out, rows.Err()
}
