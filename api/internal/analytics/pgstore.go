package analytics

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	VisitsTable     = "analytics_visits"
	EventsTable     = "analytics_events"
	VisitPagesTable = "analytics_visit_pages"
	// PartitionLock is the advisory lock every creation or removal of a raw partition holds.
	PartitionLock  = 7_310_015_001
	lastEventEvery = time.Minute
)

// PGStore keeps analytics in Postgres: raw visits and events in day partitions.
type PGStore struct {
	Pool *pgxpool.Pool
}

var _ Store = (*PGStore)(nil)

// PartitionName is the partition of a raw table that holds one day, such as analytics_events_20261012.
func PartitionName(table string, day time.Time) string {
	return table + "_" + day.Format("20060102")
}

func (s *PGStore) EnsurePartitions(ctx context.Context, from, to time.Time) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(PartitionLock)); err != nil {
			return err
		}
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			next := d.AddDate(0, 0, 1)
			for _, t := range []struct{ table, with string }{
				{VisitsTable, " WITH (fillfactor = 70, autovacuum_vacuum_scale_factor = 0.02)"},
				{EventsTable, ""},
				{VisitPagesTable, ""},
			} {
				name := PartitionName(t.table, d)
				var exists bool
				if err := tx.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", name).Scan(&exists); err != nil {
					return err
				}
				if exists {
					continue
				}
				if _, err := tx.Exec(ctx, fmt.Sprintf("CREATE TABLE %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')%s",
					name, t.table, d.Format(time.DateOnly), next.Format(time.DateOnly), t.with)); err != nil {
					return fmt.Errorf("create partition %s: %w", name, err)
				}
			}
		}
		return nil
	})
}

type visitKey3 struct {
	property uuid.UUID
	day      time.Time
	visit    int64
}

type visitRow struct {
	ws, property          uuid.UUID
	day                   time.Time
	visit, visitor        int64
	started, ended        time.Time
	active                bool
	firstView, lastView   time.Time
	hostname, entry, exit string
	views, events         int32
	interactive           bool
	engaged               int64
	info                  *VisitInfo
}

func (v *visitRow) add(e *Event) {
	if v.info == nil && e.Info != nil {
		v.info = e.Info
	}
	switch e.Kind {
	case KindEngagement:
		if e.EngagedMS != nil {
			v.engaged += int64(*e.EngagedMS)
		}
		return
	case KindPageview, KindScreen:
		if v.views == 0 || e.TS.Before(v.firstView) {
			v.firstView, v.entry, v.hostname = e.TS, e.Path, e.Hostname
		}
		if v.views == 0 || !e.TS.Before(v.lastView) {
			v.lastView, v.exit = e.TS, e.Path
		}
		v.views++
	case KindCustom:
		v.events++
		v.interactive = v.interactive || e.Interactive
		if v.views == 0 && !v.active {
			v.entry, v.exit, v.hostname = e.Path, e.Path, e.Hostname
		}
	}
	if !v.active || e.TS.Before(v.started) {
		v.started = e.TS
	}
	if !v.active || e.TS.After(v.ended) {
		v.ended = e.TS
	}
	v.active = true
}

type dayKey struct {
	ws, property uuid.UUID
	day          time.Time
}

// liveProperties are the batch's properties that still exist and are not deleted, in workspaces that
// are not deleted; events of others are not written.
func liveProperties(ctx context.Context, tx pgx.Tx, b Batch) (map[uuid.UUID]bool, error) {
	pairs := map[[2]uuid.UUID]bool{}
	for _, e := range b.Events {
		pairs[[2]uuid.UUID{e.WorkspaceID, e.PropertyID}] = true
	}
	for _, c := range b.Drops {
		pairs[[2]uuid.UUID{c.WorkspaceID, c.PropertyID}] = true
	}
	ws, props := make([]uuid.UUID, 0, len(pairs)), make([]uuid.UUID, 0, len(pairs))
	for p := range pairs {
		ws, props = append(ws, p[0]), append(props, p[1])
	}
	rows, err := tx.Query(ctx, `SELECT p.id FROM analytics_properties p
		JOIN unnest($1::uuid[], $2::uuid[]) AS u(workspace_id, id) ON p.workspace_id = u.workspace_id AND p.id = u.id
		JOIN workspaces w ON w.id = p.workspace_id
		WHERE p.deleted_at IS NULL AND w.deleted_at IS NULL`, ws, props)
	if err != nil {
		return nil, err
	}
	live, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	out := map[uuid.UUID]bool{}
	for _, id := range live {
		out[id] = true
	}
	return out, err
}

func (s *PGStore) Ingest(ctx context.Context, b Batch, now time.Time) (IngestResult, error) {
	res := IngestResult{Month: monthOf(now), Usage: map[uuid.UUID]int64{}}
	began := time.Now()
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		live, err := liveProperties(ctx, tx, b)
		if err != nil {
			return err
		}
		visits := map[visitKey3]*visitRow{}
		pages := map[visitPageKey]*visitPage{}
		accepted := map[dayKey]int64{}
		changed := foldChanges{late: map[dayKey]bool{}, engagement: map[dayKey]bool{}}
		stale := now.Add(-FoldMargin / 2)
		usage := map[uuid.UUID]int64{}
		type firstLast struct{ first, last time.Time }
		seenAt := map[[2]uuid.UUID]*firstLast{}
		rows := make([][]any, 0, len(b.Events))
		for i := range b.Events {
			e := &b.Events[i]
			if !live[e.PropertyID] {
				continue
			}
			var engaged, scroll any
			if e.EngagedMS != nil {
				engaged = *e.EngagedMS
			}
			if e.Scroll != nil {
				scroll = *e.Scroll
			}
			var props, revenue, currency any
			if len(e.Props) > 0 {
				props = e.Props
			}
			if e.Revenue != nil {
				revenue, currency = e.Revenue.Minor, e.Revenue.Currency
			}
			rows = append(rows, []any{e.WorkspaceID, e.PropertyID, e.Day, e.TS, e.Visit, e.Visitor, int16(e.Kind), e.Name, e.Hostname, e.Path,
				props, revenue, currency, engaged, scroll, e.Interactive})
			accepted[dayKey{e.WorkspaceID, e.PropertyID, e.Day}]++
			if !e.TS.After(stale) {
				changed.late[dayKey{e.WorkspaceID, e.PropertyID, e.Day}] = true
			}
			usage[e.WorkspaceID]++
			pk := [2]uuid.UUID{e.WorkspaceID, e.PropertyID}
			if fl := seenAt[pk]; fl == nil {
				seenAt[pk] = &firstLast{e.TS, e.TS}
			} else {
				fl.first, fl.last = minTime(fl.first, e.TS), maxTime(fl.last, e.TS)
			}
			k := visitKey3{e.PropertyID, e.Day, e.Visit}
			v := visits[k]
			if v == nil {
				v = &visitRow{ws: e.WorkspaceID, property: e.PropertyID, day: e.Day, visit: e.Visit, visitor: e.Visitor,
					hostname: e.Hostname, entry: e.Path, exit: e.Path}
				visits[k] = v
			}
			v.add(e)
			pk3 := visitPageKey{visitKey3: k, path: e.Path}
			vp := pages[pk3]
			if vp == nil {
				vp = &visitPage{ws: e.WorkspaceID}
				pages[pk3] = vp
			}
			vp.add(e)
		}
		drops := slices.DeleteFunc(slices.Clone(b.Drops), func(c Count) bool { return !live[c.PropertyID] })
		if len(rows) > 0 {
			if _, err := tx.CopyFrom(ctx, pgx.Identifier{EventsTable}, []string{"workspace_id", "property_id", "day", "ts", "visit", "visitor",
				"kind", "name", "hostname", "path", "props", "revenue_minor", "currency", "engaged_ms", "scroll", "interactive"},
				pgx.CopyFromRows(rows)); err != nil {
				return fmt.Errorf("copy events: %w", err)
			}
		}
		old, err := upsertVisits(ctx, tx, visits, stale.Add(-VisitGap))
		if err != nil {
			return err
		}
		if err := changed.oldVisits(ctx, tx, visits, pages, old); err != nil {
			return err
		}
		if err := upsertVisitPages(ctx, tx, pages); err != nil {
			return err
		}
		if err := markBots(ctx, tx, b.Bots, stale.Add(-VisitGap), changed.late); err != nil {
			return err
		}
		if err := addCounts(ctx, tx, accepted, drops); err != nil {
			return err
		}
		if res.Late, err = markDays(ctx, tx, accepted); err != nil {
			return err
		}
		if err := addUsage(ctx, tx, res.Month, usage, res.Usage); err != nil {
			return err
		}
		for _, pk := range slices.SortedFunc(maps.Keys(seenAt), func(a, b [2]uuid.UUID) int { return a[1].Compare(b[1]) }) {
			fl := seenAt[pk]
			if _, err := tx.Exec(ctx, `UPDATE analytics_properties SET last_event_at = $3, first_event_at = coalesce(first_event_at, $4)
				WHERE workspace_id = $1 AND id = $2 AND (last_event_at IS NULL OR last_event_at < $5)`,
				pk[0], pk[1], fl.last, fl.first, fl.last.Add(-lastEventEvery)); err != nil {
				return err
			}
		}
		if time.Since(began) > FoldMargin/4 {
			for k := range accepted {
				changed.late[k] = true
			}
		}
		return changed.bump(ctx, tx)
	})
	return res, err
}

// upsertVisits writes the batch's visits and returns those of its engagements that joined a visit
// which had ended by endedBy.
func upsertVisits(ctx context.Context, tx pgx.Tx, visits map[visitKey3]*visitRow, endedBy time.Time) (map[visitKey3]bool, error) {
	list := make([]*visitRow, 0, len(visits))
	for _, v := range visits {
		list = append(list, v)
	}
	slices.SortFunc(list, func(a, b *visitRow) int {
		return cmp.Or(a.property.Compare(b.property), a.day.Compare(b.day), cmp.Compare(a.visit, b.visit))
	})
	var ins, eng, joined []*visitRow
	for _, v := range list {
		if v.active {
			ins = append(ins, v)
		} else if v.engaged > 0 {
			eng = append(eng, v)
		} else {
			joined = append(joined, v)
		}
	}
	old := map[visitKey3]bool{}
	if len(ins) > 0 {
		n := len(ins)
		var (
			ws, props                                                    = make([]uuid.UUID, n), make([]uuid.UUID, n)
			days, started, ended                                         = make([]time.Time, n), make([]time.Time, n), make([]time.Time, n)
			visit, visitor, engaged                                      = make([]int64, n), make([]int64, n), make([]int64, n)
			views, events                                                = make([]int32, n), make([]int32, n)
			interactive                                                  = make([]bool, n)
			hostname, entry, exit, source, refHost, refPath              = make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n)
			utmSource, utmMedium, utmCampaign, utmTerm, utmContent       = make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n)
			country, region, city, browser, osName, language, appVersion = make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n)
			channel, device, browserMajor, osMajor, screen, platform     = make([]int16, n), make([]int16, n), make([]int16, n), make([]int16, n), make([]int16, n), make([]int16, n)
		)
		for i, v := range ins {
			info := v.info
			if info == nil {
				info = &VisitInfo{Channel: ChannelDirect, Platform: PlatformWeb}
			}
			ws[i], props[i], days[i], visit[i], visitor[i] = v.ws, v.property, v.day, v.visit, v.visitor
			started[i], ended[i], hostname[i], entry[i], exit[i] = v.started, v.ended, v.hostname, v.entry, v.exit
			views[i], events[i], interactive[i], engaged[i] = v.views, v.events, v.interactive, v.engaged
			channel[i], source[i], refHost[i], refPath[i] = int16(info.Channel), info.Source, info.ReferrerHost, info.ReferrerPath
			a := info.Acquisition
			utmSource[i], utmMedium[i], utmCampaign[i], utmTerm[i], utmContent[i] = a.Source, a.Medium, a.Campaign, a.Term, a.Content
			country[i], region[i], city[i], device[i] = info.Country, info.Region, info.City, info.Device
			browser[i], browserMajor[i], osName[i], osMajor[i] = info.Browser, info.BrowserMajor, info.OS, info.OSMajor
			screen[i], language[i], platform[i], appVersion[i] = info.Screen, info.Language, int16(info.Platform), info.AppVersion
		}
		var infoProps []uuid.UUID
		var infoDays []time.Time
		var infoVisits []int64
		var infoN []int32
		for _, v := range ins {
			if v.info != nil {
				infoProps, infoDays, infoVisits, infoN = append(infoProps, v.property), append(infoDays, v.day), append(infoVisits, v.visit), append(infoN, v.views+v.events)
			}
		}
		rows, err := tx.Query(ctx, `
WITH up AS (
INSERT INTO analytics_visits (workspace_id, property_id, day, visit, visitor, started_at, ended_at, hostname, entry_path, exit_path,
    views, events, interactive, engaged_ms, channel, source, referrer_host, referrer_path, utm_source, utm_medium, utm_campaign,
    utm_term, utm_content, country, region, city, device, browser, browser_major, os, os_major, screen, language, platform, app_version)
SELECT * FROM unnest($1::uuid[], $2::uuid[], $3::date[], $4::bigint[], $5::bigint[], $6::timestamptz[], $7::timestamptz[], $8::text[],
    $9::text[], $10::text[], $11::integer[], $12::integer[], $13::boolean[], $14::bigint[], $15::smallint[], $16::text[], $17::text[],
    $18::text[], $19::text[], $20::text[], $21::text[], $22::text[], $23::text[], $24::text[], $25::text[], $26::text[], $27::smallint[],
    $28::text[], $29::smallint[], $30::text[], $31::smallint[], $32::smallint[], $33::text[], $34::smallint[], $35::text[])
ON CONFLICT (property_id, day, visit) DO UPDATE SET
    started_at = least(analytics_visits.started_at, EXCLUDED.started_at),
    entry_path = CASE WHEN EXCLUDED.started_at < analytics_visits.started_at THEN EXCLUDED.entry_path ELSE analytics_visits.entry_path END,
    ended_at = greatest(analytics_visits.ended_at, EXCLUDED.ended_at),
    exit_path = CASE WHEN EXCLUDED.ended_at >= analytics_visits.ended_at THEN EXCLUDED.exit_path ELSE analytics_visits.exit_path END,
    views = analytics_visits.views + EXCLUDED.views,
    events = analytics_visits.events + EXCLUDED.events,
    interactive = analytics_visits.interactive OR EXCLUDED.interactive,
    engaged_ms = analytics_visits.engaged_ms + EXCLUDED.engaged_ms
RETURNING property_id, day, visit, views + events AS n)
SELECT up.property_id, up.day, up.visit FROM up
JOIN unnest($36::uuid[], $37::date[], $38::bigint[], $39::integer[]) AS i(property_id, day, visit, n)
    ON up.property_id = i.property_id AND up.day = i.day AND up.visit = i.visit
WHERE up.n > i.n`,
			ws, props, days, visit, visitor, started, ended, hostname, entry, exit, views, events, interactive, engaged, channel, source, refHost,
			refPath, utmSource, utmMedium, utmCampaign, utmTerm, utmContent, country, region, city, device, browser, browserMajor, osName,
			osMajor, screen, language, platform, appVersion, infoProps, infoDays, infoVisits, infoN)
		if err != nil {
			return nil, fmt.Errorf("upsert visits: %w", err)
		}
		describe, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (visitKey3, error) {
			var k visitKey3
			err := row.Scan(&k.property, &k.day, &k.visit)
			k.day = k.day.UTC()
			return k, err
		})
		if err != nil {
			return nil, fmt.Errorf("upsert visits: %w", err)
		}
		if err := describeJoinedVisits(ctx, tx, describe, ins); err != nil {
			return nil, err
		}
	}
	if len(eng) > 0 {
		n := len(eng)
		ws, props, days, visit, engaged := make([]uuid.UUID, n), make([]uuid.UUID, n), make([]time.Time, n), make([]int64, n), make([]int64, n)
		for i, v := range eng {
			ws[i], props[i], days[i], visit[i], engaged[i] = v.ws, v.property, v.day, v.visit, v.engaged
		}
		rows, err := tx.Query(ctx, `
UPDATE analytics_visits v SET engaged_ms = v.engaged_ms + u.engaged
FROM unnest($1::uuid[], $2::uuid[], $3::date[], $4::bigint[], $5::bigint[]) AS u(workspace_id, property_id, day, visit, engaged)
WHERE v.workspace_id = u.workspace_id AND v.property_id = u.property_id AND v.day = u.day AND v.visit = u.visit
RETURNING v.property_id, v.day, v.visit, v.ended_at <= $6`,
			ws, props, days, visit, engaged, endedBy)
		if err != nil {
			return nil, fmt.Errorf("add engagement: %w", err)
		}
		if err := collectOld(rows, old); err != nil {
			return nil, fmt.Errorf("add engagement: %w", err)
		}
	}
	if len(joined) > 0 {
		n := len(joined)
		ws, props, days, visit := make([]uuid.UUID, n), make([]uuid.UUID, n), make([]time.Time, n), make([]int64, n)
		for i, v := range joined {
			ws[i], props[i], days[i], visit[i] = v.ws, v.property, v.day, v.visit
		}
		rows, err := tx.Query(ctx, `
SELECT v.property_id, v.day, v.visit, v.ended_at <= $5
FROM analytics_visits v JOIN unnest($1::uuid[], $2::uuid[], $3::date[], $4::bigint[]) AS u(workspace_id, property_id, day, visit)
    ON v.workspace_id = u.workspace_id AND v.property_id = u.property_id AND v.day = u.day AND v.visit = u.visit`,
			ws, props, days, visit, endedBy)
		if err != nil {
			return nil, fmt.Errorf("engaged visits: %w", err)
		}
		if err := collectOld(rows, old); err != nil {
			return nil, fmt.Errorf("engaged visits: %w", err)
		}
	}
	return old, nil
}

// describeJoinedVisits copies the description a visit's first event carries onto a visit that was
// already written: another process joined the visit and flushed before the one that started it.
func describeJoinedVisits(ctx context.Context, tx pgx.Tx, keys []visitKey3, ins []*visitRow) error {
	if len(keys) == 0 {
		return nil
	}
	byKey := make(map[visitKey3]*visitRow, len(ins))
	for _, v := range ins {
		byKey[visitKey3{v.property, v.day, v.visit}] = v
	}
	var (
		ws, props                                              []uuid.UUID
		days                                                   []time.Time
		visit                                                  []int64
		source, refHost, refPath                               []string
		utmSource, utmMedium, utmCampaign, utmTerm, utmContent []string
		country, region, city, browser, osName, language, app  []string
		channel, device, browserMajor, osMajor, screen, plat   []int16
	)
	for _, k := range keys {
		v := byKey[k]
		if v == nil || v.info == nil {
			continue
		}
		i, a := v.info, v.info.Acquisition
		ws, props, days, visit = append(ws, v.ws), append(props, v.property), append(days, v.day), append(visit, v.visit)
		source, refHost, refPath = append(source, i.Source), append(refHost, i.ReferrerHost), append(refPath, i.ReferrerPath)
		utmSource, utmMedium, utmCampaign = append(utmSource, a.Source), append(utmMedium, a.Medium), append(utmCampaign, a.Campaign)
		utmTerm, utmContent = append(utmTerm, a.Term), append(utmContent, a.Content)
		country, region, city = append(country, i.Country), append(region, i.Region), append(city, i.City)
		browser, osName, language, app = append(browser, i.Browser), append(osName, i.OS), append(language, i.Language), append(app, i.AppVersion)
		channel, device, browserMajor = append(channel, int16(i.Channel)), append(device, i.Device), append(browserMajor, i.BrowserMajor)
		osMajor, screen, plat = append(osMajor, i.OSMajor), append(screen, i.Screen), append(plat, int16(i.Platform))
	}
	if len(visit) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
UPDATE analytics_visits v SET channel = u.channel, source = u.source, referrer_host = u.referrer_host, referrer_path = u.referrer_path,
    utm_source = u.utm_source, utm_medium = u.utm_medium, utm_campaign = u.utm_campaign, utm_term = u.utm_term, utm_content = u.utm_content,
    country = u.country, region = u.region, city = u.city, device = u.device, browser = u.browser, browser_major = u.browser_major,
    os = u.os, os_major = u.os_major, screen = u.screen, language = u.language, platform = u.platform, app_version = u.app_version
FROM unnest($1::uuid[], $2::uuid[], $3::date[], $4::bigint[], $5::smallint[], $6::text[], $7::text[], $8::text[], $9::text[], $10::text[],
    $11::text[], $12::text[], $13::text[], $14::text[], $15::text[], $16::text[], $17::smallint[], $18::text[], $19::smallint[], $20::text[],
    $21::smallint[], $22::smallint[], $23::text[], $24::smallint[], $25::text[])
    AS u(workspace_id, property_id, day, visit, channel, source, referrer_host, referrer_path, utm_source, utm_medium, utm_campaign, utm_term,
    utm_content, country, region, city, device, browser, browser_major, os, os_major, screen, language, platform, app_version)
WHERE v.workspace_id = u.workspace_id AND v.property_id = u.property_id AND v.day = u.day AND v.visit = u.visit`,
		ws, props, days, visit, channel, source, refHost, refPath, utmSource, utmMedium, utmCampaign, utmTerm, utmContent,
		country, region, city, device, browser, browserMajor, osName, osMajor, screen, language, plat, app); err != nil {
		return fmt.Errorf("describe joined visits: %w", err)
	}
	return nil
}

func collectOld(rows pgx.Rows, old map[visitKey3]bool) error {
	defer rows.Close()
	for rows.Next() {
		var k visitKey3
		var ended bool
		if err := rows.Scan(&k.property, &k.day, &k.visit, &ended); err != nil {
			return err
		}
		if ended {
			old[visitKey3{k.property, k.day.UTC(), k.visit}] = true
		}
	}
	return rows.Err()
}

// foldChanges are the days whose folds no longer hold: late for every query, engagement for the
// engagement metrics.
type foldChanges struct {
	late, engagement map[dayKey]bool
}

// oldVisits records engagements that joined a visit which had ended before any fold's cut: they
// change the visit's engagement, and a path the visit had no row for changes its page numbers.
func (c foldChanges) oldVisits(ctx context.Context, tx pgx.Tx, visits map[visitKey3]*visitRow, pages map[visitPageKey]*visitPage, old map[visitKey3]bool) error {
	if len(old) == 0 {
		return nil
	}
	var ws, props []uuid.UUID
	var days []time.Time
	var visit []int64
	var paths []string
	for k := range pages {
		if !old[k.visitKey3] {
			continue
		}
		v := visits[k.visitKey3]
		c.engagement[dayKey{v.ws, k.property, k.day}] = true
		ws, props, days, visit, paths = append(ws, v.ws), append(props, k.property), append(days, k.day), append(visit, k.visit), append(paths, k.path)
	}
	rows, err := tx.Query(ctx, `
SELECT u.workspace_id, u.property_id, u.day FROM unnest($1::uuid[], $2::uuid[], $3::date[], $4::bigint[], $5::text[]) AS u(workspace_id, property_id, day, visit, path)
WHERE NOT EXISTS (SELECT 1 FROM analytics_visit_pages p WHERE p.workspace_id = u.workspace_id AND p.property_id = u.property_id
    AND p.day = u.day AND p.visit = u.visit AND p.path = u.path)`, ws, props, days, visit, paths)
	if err != nil {
		return fmt.Errorf("new paths of old visits: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k dayKey
		if err := rows.Scan(&k.ws, &k.property, &k.day); err != nil {
			return err
		}
		c.late[dayKey{k.ws, k.property, k.day.UTC()}] = true
	}
	return rows.Err()
}

// bump moves the sequences of the changed days, in the transaction that changed them, so a fold
// that did not see the change sees its sequence move.
func (c foldChanges) bump(ctx context.Context, tx pgx.Tx) error {
	all := map[dayKey]bool{}
	for k := range c.late {
		all[k] = true
	}
	for k := range c.engagement {
		all[k] = true
	}
	if len(all) == 0 {
		return nil
	}
	keys := slices.SortedFunc(maps.Keys(all), func(a, b dayKey) int {
		return cmp.Or(a.property.Compare(b.property), a.day.Compare(b.day))
	})
	n := len(keys)
	ws, props, days, late, eng := make([]uuid.UUID, n), make([]uuid.UUID, n), make([]time.Time, n), make([]bool, n), make([]bool, n)
	for i, k := range keys {
		ws[i], props[i], days[i], late[i], eng[i] = k.ws, k.property, k.day, c.late[k], c.engagement[k]
	}
	_, err := tx.Exec(ctx, `
UPDATE analytics_rollup_days d SET late_seq = d.late_seq + u.late::int, engagement_seq = d.engagement_seq + u.eng::int
FROM unnest($1::uuid[], $2::uuid[], $3::date[], $4::boolean[], $5::boolean[]) AS u(workspace_id, property_id, day, late, eng)
WHERE d.workspace_id = u.workspace_id AND d.property_id = u.property_id AND d.day = u.day`, ws, props, days, late, eng)
	if err != nil {
		return fmt.Errorf("fold changes: %w", err)
	}
	return nil
}

type visitPageKey struct {
	visitKey3
	path string
}

// visitPage is a visit's numbers on one path, as the page breakdowns sum them.
type visitPage struct {
	ws                                   uuid.UUID
	views, events, engViews, scrollViews int32
	engMS, scrollSum                     int64
}

func (p *visitPage) add(e *Event) {
	switch e.Kind {
	case KindPageview:
		p.views++
	case KindCustom:
		p.events++
	case KindEngagement:
		p.engViews++
		if e.EngagedMS != nil {
			p.engMS += int64(*e.EngagedMS)
		}
		if e.Scroll != nil {
			p.scrollViews++
			p.scrollSum += int64(*e.Scroll)
		}
	}
}

func upsertVisitPages(ctx context.Context, tx pgx.Tx, pages map[visitPageKey]*visitPage) error {
	if len(pages) == 0 {
		return nil
	}
	keys := slices.SortedFunc(maps.Keys(pages), func(a, b visitPageKey) int {
		return cmp.Or(a.property.Compare(b.property), a.day.Compare(b.day), cmp.Compare(a.visit, b.visit), strings.Compare(a.path, b.path))
	})
	n := len(keys)
	var (
		ws, props                            = make([]uuid.UUID, n), make([]uuid.UUID, n)
		days                                 = make([]time.Time, n)
		visit, engMS, scrollSum              = make([]int64, n), make([]int64, n), make([]int64, n)
		paths                                = make([]string, n)
		views, events, engViews, scrollViews = make([]int32, n), make([]int32, n), make([]int32, n), make([]int32, n)
	)
	for i, k := range keys {
		p := pages[k]
		ws[i], props[i], days[i], visit[i], paths[i] = p.ws, k.property, k.day, k.visit, k.path
		views[i], events[i], engMS[i], engViews[i], scrollSum[i], scrollViews[i] = p.views, p.events, p.engMS, p.engViews, p.scrollSum, p.scrollViews
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO analytics_visit_pages (workspace_id, property_id, day, visit, path, views, events, eng_ms, eng_views, scroll_sum, scroll_views)
SELECT * FROM unnest($1::uuid[], $2::uuid[], $3::date[], $4::bigint[], $5::text[], $6::integer[], $7::integer[], $8::bigint[], $9::integer[],
    $10::bigint[], $11::integer[])
ON CONFLICT (property_id, day, visit, path) DO UPDATE SET
    views = analytics_visit_pages.views + EXCLUDED.views,
    events = analytics_visit_pages.events + EXCLUDED.events,
    eng_ms = analytics_visit_pages.eng_ms + EXCLUDED.eng_ms,
    eng_views = analytics_visit_pages.eng_views + EXCLUDED.eng_views,
    scroll_sum = analytics_visit_pages.scroll_sum + EXCLUDED.scroll_sum,
    scroll_views = analytics_visit_pages.scroll_views + EXCLUDED.scroll_views`,
		ws, props, days, visit, paths, views, events, engMS, engViews, scrollSum, scrollViews); err != nil {
		return fmt.Errorf("upsert visit pages: %w", err)
	}
	return nil
}

// markBots marks the bots' visits of their day; a bot with a visit that had ended by endedBy makes
// its day late.
func markBots(ctx context.Context, tx pgx.Tx, bots []BotVisitor, endedBy time.Time, late map[dayKey]bool) error {
	if len(bots) == 0 {
		return nil
	}
	n := len(bots)
	ws, props, days, visitors := make([]uuid.UUID, n), make([]uuid.UUID, n), make([]time.Time, n), make([]int64, n)
	for i, b := range bots {
		ws[i], props[i], days[i], visitors[i] = b.WorkspaceID, b.PropertyID, b.Day, b.Visitor
	}
	rows, err := tx.Query(ctx, `
UPDATE analytics_visits v SET bot = true
FROM unnest($1::uuid[], $2::uuid[], $3::date[], $4::bigint[]) AS u(workspace_id, property_id, day, visitor)
WHERE v.workspace_id = u.workspace_id AND v.property_id = u.property_id AND v.day = u.day AND v.visitor = u.visitor AND NOT v.bot
RETURNING v.workspace_id, v.property_id, v.day, v.ended_at <= $5`,
		ws, props, days, visitors, endedBy)
	if err != nil {
		return fmt.Errorf("mark bots: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k dayKey
		var ended bool
		if err := rows.Scan(&k.ws, &k.property, &k.day, &ended); err != nil {
			return fmt.Errorf("mark bots: %w", err)
		}
		if ended {
			late[dayKey{k.ws, k.property, k.day.UTC()}] = true
		}
	}
	return rows.Err()
}

func addCounts(ctx context.Context, tx pgx.Tx, accepted map[dayKey]int64, drops []Count) error {
	counts := slices.Clone(drops)
	for k, n := range accepted {
		counts = append(counts, Count{WorkspaceID: k.ws, PropertyID: k.property, Day: k.day, Reason: ReasonAccepted, N: n})
	}
	if len(counts) == 0 {
		return nil
	}
	slices.SortFunc(counts, func(a, b Count) int {
		return cmp.Or(a.PropertyID.Compare(b.PropertyID), a.Day.Compare(b.Day), cmp.Compare(a.Reason, b.Reason))
	})
	n := len(counts)
	ws, props, days, reasons, events := make([]uuid.UUID, n), make([]uuid.UUID, n), make([]time.Time, n), make([]string, n), make([]int64, n)
	for i, c := range counts {
		ws[i], props[i], days[i], reasons[i], events[i] = c.WorkspaceID, c.PropertyID, c.Day, c.Reason, c.N
	}
	_, err := tx.Exec(ctx, `
INSERT INTO analytics_ingest_stats (workspace_id, property_id, day, reason, events)
SELECT u.workspace_id, u.property_id, u.day, u.reason, sum(u.events)
FROM unnest($1::uuid[], $2::uuid[], $3::date[], $4::text[], $5::bigint[]) AS u(workspace_id, property_id, day, reason, events)
GROUP BY u.workspace_id, u.property_id, u.day, u.reason
ORDER BY u.property_id, u.day, u.reason
ON CONFLICT (property_id, day, reason) DO UPDATE SET events = analytics_ingest_stats.events + EXCLUDED.events`,
		ws, props, days, reasons, events)
	if err != nil {
		return fmt.Errorf("ingest stats: %w", err)
	}
	return nil
}

// markDays records the days that have raw rows, for the rollups, and marks those already rolled
// up dirty; it returns how many events arrived for such days.
func markDays(ctx context.Context, tx pgx.Tx, accepted map[dayKey]int64) (int64, error) {
	if len(accepted) == 0 {
		return 0, nil
	}
	keys := slices.SortedFunc(maps.Keys(accepted), func(a, b dayKey) int {
		return cmp.Or(a.property.Compare(b.property), a.day.Compare(b.day))
	})
	n := len(keys)
	ws, props, days := make([]uuid.UUID, n), make([]uuid.UUID, n), make([]time.Time, n)
	for i, k := range keys {
		ws[i], props[i], days[i] = k.ws, k.property, k.day
	}
	rows, err := tx.Query(ctx, `
INSERT INTO analytics_rollup_days (workspace_id, property_id, day)
SELECT u.workspace_id, u.property_id, u.day FROM unnest($1::uuid[], $2::uuid[], $3::date[]) AS u(workspace_id, property_id, day)
ON CONFLICT (property_id, day) DO UPDATE SET dirty = true
WHERE analytics_rollup_days.built_at IS NOT NULL AND NOT analytics_rollup_days.dirty
RETURNING workspace_id, property_id, day, xmax <> 0`, ws, props, days)
	if err != nil {
		return 0, fmt.Errorf("rollup days: %w", err)
	}
	var late int64
	for rows.Next() {
		var k dayKey
		var updated bool
		if err := rows.Scan(&k.ws, &k.property, &k.day, &updated); err != nil {
			return 0, err
		}
		if updated {
			late += accepted[dayKey{k.ws, k.property, k.day.UTC()}]
		}
	}
	return late, rows.Err()
}

func addUsage(ctx context.Context, tx pgx.Tx, month time.Time, usage map[uuid.UUID]int64, out map[uuid.UUID]int64) error {
	for _, ws := range slices.SortedFunc(maps.Keys(usage), func(a, b uuid.UUID) int { return a.Compare(b) }) {
		var total int64
		err := tx.QueryRow(ctx, `
INSERT INTO usage_counters (workspace_id, month, analytics_events) VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, month) DO UPDATE SET analytics_events = usage_counters.analytics_events + EXCLUDED.analytics_events
RETURNING analytics_events`, ws, month, usage[ws]).Scan(&total)
		if err != nil {
			return fmt.Errorf("usage: %w", err)
		}
		out[ws] = total
	}
	return nil
}

func (s *PGStore) Salt(ctx context.Context, workspaceID, propertyID uuid.UUID, day time.Time, fresh []byte, expires time.Time) ([]byte, time.Time, error) {
	if _, err := s.Pool.Exec(ctx, `INSERT INTO analytics_salts (workspace_id, property_id, day, salt, expires_at) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (property_id, day) DO NOTHING`, workspaceID, propertyID, day, fresh, expires); err != nil {
		return nil, time.Time{}, err
	}
	var salt []byte
	var exp time.Time
	err := s.Pool.QueryRow(ctx, "SELECT salt, expires_at FROM analytics_salts WHERE workspace_id = $1 AND property_id = $2 AND day = $3",
		workspaceID, propertyID, day).Scan(&salt, &exp)
	return salt, exp, err
}

func (s *PGStore) LatestVisit(ctx context.Context, workspaceID, propertyID uuid.UUID, day time.Time, visitor int64) (LatestVisit, bool, error) {
	var out LatestVisit
	err := s.Pool.QueryRow(ctx, `SELECT visit, ended_at, bot FROM analytics_visits
		WHERE workspace_id = $1 AND property_id = $2 AND day = $3 AND visitor = $4 ORDER BY ended_at DESC LIMIT 1`,
		workspaceID, propertyID, day, visitor).Scan(&out.Visit, &out.Ended, &out.Bot)
	if errors.Is(err, pgx.ErrNoRows) {
		return LatestVisit{}, false, nil
	}
	return out, err == nil, err
}

func (s *PGStore) ClaimVisit(ctx context.Context, workspaceID, propertyID uuid.UUID, day time.Time, visitor, visit int64, now, staleBefore time.Time) (int64, bool, error) {
	for range 3 {
		var out int64
		// A claim lost in a crash costs at most one split visit, so its commit does not wait for the WAL flush.
		err := s.Pool.QueryRow(ctx, `INSERT INTO analytics_visit_claims (workspace_id, property_id, day, visitor, visit, claimed_at)
			SELECT $1, $2, $3, $4, $5, $6 FROM (SELECT set_config('synchronous_commit', 'off', true)) AS async
			ON CONFLICT (property_id, day, visitor) DO NOTHING RETURNING visit`,
			workspaceID, propertyID, day, visitor, visit, now).Scan(&out)
		if err == nil {
			return out, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return 0, false, err
		}
		err = s.Pool.QueryRow(ctx, `UPDATE analytics_visit_claims SET visit = $5, claimed_at = $6
			FROM (SELECT set_config('synchronous_commit', 'off', true)) AS async WHERE workspace_id = $1 AND property_id = $2 AND day = $3 AND visitor = $4 AND claimed_at < $7 RETURNING visit`,
			workspaceID, propertyID, day, visitor, visit, now, staleBefore).Scan(&out)
		if err == nil {
			return out, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return 0, false, err
		}
		err = s.Pool.QueryRow(ctx, `SELECT visit FROM analytics_visit_claims WHERE workspace_id = $1 AND property_id = $2 AND day = $3 AND visitor = $4`,
			workspaceID, propertyID, day, visitor).Scan(&out)
		if err == nil {
			return out, false, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return 0, false, err
		}
	}
	return 0, false, errors.New("analytics: visit claim kept changing")
}

func (s *PGStore) ClaimedVisit(ctx context.Context, workspaceID, propertyID uuid.UUID, day time.Time, visitor int64, since time.Time) (int64, bool, error) {
	var out int64
	err := s.Pool.QueryRow(ctx, `SELECT visit FROM analytics_visit_claims
		WHERE workspace_id = $1 AND property_id = $2 AND day = $3 AND visitor = $4 AND claimed_at >= $5`,
		workspaceID, propertyID, day, visitor, since).Scan(&out)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return out, err == nil, err
}

func (s *PGStore) VisitorsToday(ctx context.Context, workspaceID uuid.UUID, days map[uuid.UUID]time.Time) (map[uuid.UUID]int64, error) {
	out := map[uuid.UUID]int64{}
	if len(days) == 0 {
		return out, nil
	}
	props, ds := make([]uuid.UUID, 0, len(days)), make([]time.Time, 0, len(days))
	for p, d := range days {
		props, ds = append(props, p), append(ds, d)
	}
	rows, err := s.Pool.Query(ctx, `SELECT v.property_id, count(DISTINCT v.visitor) FROM analytics_visits v
		JOIN unnest($2::uuid[], $3::date[]) AS u(property_id, day) ON v.property_id = u.property_id AND v.day = u.day
		WHERE v.workspace_id = $1 AND NOT v.bot GROUP BY v.property_id`, workspaceID, props, ds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p uuid.UUID
		var n int64
		if err := rows.Scan(&p, &n); err != nil {
			return nil, err
		}
		out[p] = n
	}
	return out, rows.Err()
}

// DeleteRaw deletes up to limit rows of each raw table of a property's day; the day's folds stop
// counting.
func (s *PGStore) DeleteRaw(ctx context.Context, workspaceID, propertyID uuid.UUID, day time.Time, limit int) (int64, error) {
	var total int64
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		total = 0
		for _, table := range []string{EventsTable, VisitPagesTable, VisitsTable} {
			name := PartitionName(table, day)
			var exists bool
			if err := tx.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", name).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				continue
			}
			tag, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %[1]s WHERE ctid = ANY(ARRAY(
				SELECT ctid FROM %[1]s WHERE workspace_id = $1 AND property_id = $2 LIMIT $3))`, name), workspaceID, propertyID, limit)
			if err != nil {
				return err
			}
			total += tag.RowsAffected()
		}
		if total == 0 {
			return nil
		}
		_, err := tx.Exec(ctx, `UPDATE analytics_rollup_days SET late_seq = late_seq + 1 WHERE workspace_id = $1 AND property_id = $2 AND day = $3`,
			workspaceID, propertyID, day)
		return err
	})
	return total, err
}

func monthOf(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
