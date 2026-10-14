package monitor

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/productdevbook/yuva/api/internal/analytics"
)

const (
	MinuteTable  = "monitor_minute"
	HourTable    = "monitor_hour"
	DimHourTable = "monitor_dim_hour"
	namesLock    = 7_310_016
)

// PGStore keeps an API's rows in Postgres.
type PGStore struct {
	Pool *pgxpool.Pool
}

var _ Store = (*PGStore)(nil)

// MinutePartition is the partition of monitor_minute that holds one UTC day; HourPartition the
// partition of an hourly table that holds one month.
func MinutePartition(day time.Time) string { return MinuteTable + "_" + day.Format("20060102") }

func HourPartition(table string, month time.Time) string { return table + "_" + month.Format("200601") }

func (s *PGStore) EnsureMinutePartitions(ctx context.Context, from, to time.Time) error {
	var parts []partition
	for d := utcDay(from); !d.After(utcDay(to)); d = d.AddDate(0, 0, 1) {
		parts = append(parts, partition{MinuteTable, MinutePartition(d), d, d.AddDate(0, 0, 1)})
	}
	return s.ensure(ctx, parts)
}

func (s *PGStore) EnsureHourPartitions(ctx context.Context, from, to time.Time) error {
	var parts []partition
	for m := monthOf(from); !m.After(monthOf(to)); m = m.AddDate(0, 1, 0) {
		for _, t := range []string{HourTable, DimHourTable} {
			parts = append(parts, partition{t, HourPartition(t, m), m, m.AddDate(0, 1, 0)})
		}
	}
	return s.ensure(ctx, parts)
}

type partition struct {
	parent, name string
	from, to     time.Time
}

func (s *PGStore) ensure(ctx context.Context, parts []partition) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", int64(analytics.PartitionLock)); err != nil {
			return err
		}
		for _, p := range parts {
			var exists bool
			if err := tx.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", p.name).Scan(&exists); err != nil {
				return err
			}
			if exists {
				continue
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf("CREATE TABLE %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s') WITH (fillfactor = 70)",
				p.name, p.parent, p.from.Format(time.RFC3339), p.to.Format(time.RFC3339))); err != nil {
				return fmt.Errorf("create partition %s: %w", p.name, err)
			}
		}
		return nil
	})
}

// Names finds or adds each value's id under a lock per property and kind, so two processes never
// pass a cap together. (other) and (unmatched) never count towards a cap; services are capped per
// workspace.
func (s *PGStore) Names(ctx context.Context, ws, prop uuid.UUID, reqs []NameRequest, now time.Time) ([]int32, error) {
	out := make([]int32, len(reqs))
	byKind := map[string][]int{}
	for i, r := range reqs {
		byKind[r.Kind] = append(byKind[r.Kind], i)
	}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		for _, kind := range slices.Sorted(maps.Keys(byKind)) {
			idx := byKind[kind]
			values := make([]string, 0, len(idx))
			for _, i := range idx {
				values = append(values, reqs[i].Value)
			}
			ids, err := names(ctx, tx, ws, prop, kind, values, inferredOf(reqs, idx), now)
			if err != nil {
				return err
			}
			for _, i := range idx {
				out[i] = ids[reqs[i].Value]
			}
		}
		return nil
	})
	return out, err
}

func inferredOf(reqs []NameRequest, idx []int) map[string]bool {
	out := map[string]bool{}
	for _, i := range idx {
		out[reqs[i].Value] = reqs[i].Inferred
	}
	return out
}

func special(v string) bool { return v == OtherValue || v == Unmatched }

func names(ctx context.Context, tx pgx.Tx, ws, prop uuid.UUID, kind string, values []string, inferred map[string]bool, now time.Time) (map[string]int32, error) {
	table, col, where := "monitor_names", "value", " AND kind = $3"
	args := []any{ws, prop, kind}
	if kind == KindRoute {
		table, col, where = "monitor_routes", "pattern", ""
		args = args[:2]
	}
	find := func(vs []string) (map[string]int32, error) {
		rows, err := tx.Query(ctx, fmt.Sprintf("SELECT %s, id FROM %s WHERE workspace_id = $1 AND property_id = $2%s AND %s = ANY($%d)",
			col, table, where, col, len(args)+1), append(slices.Clone(args), vs)...)
		if err != nil {
			return nil, err
		}
		out := map[string]int32{}
		for rows.Next() {
			var v string
			var id int32
			if err := rows.Scan(&v, &id); err != nil {
				rows.Close()
				return nil, err
			}
			out[v] = id
		}
		rows.Close()
		return out, rows.Err()
	}
	got, err := find(values)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, v := range values {
		if _, ok := got[v]; !ok && !slices.Contains(missing, v) {
			missing = append(missing, v)
		}
	}
	if len(missing) == 0 {
		return got, nil
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1, hashtext($2))", int32(namesLock), prop.String()+"/"+kind); err != nil {
		return nil, err
	}
	if got, err = find(values); err != nil {
		return nil, err
	}
	missing = slices.DeleteFunc(missing, func(v string) bool { _, ok := got[v]; return ok })
	if len(missing) == 0 {
		return got, nil
	}
	var used int
	switch kind {
	case KindRoute:
		err = tx.QueryRow(ctx, `SELECT count(*) FROM monitor_routes WHERE workspace_id = $1 AND property_id = $2 AND pattern NOT IN ('(other)', '(unmatched)')`,
			ws, prop).Scan(&used)
	case KindService:
		err = tx.QueryRow(ctx, `SELECT count(DISTINCT value) FROM monitor_names WHERE workspace_id = $1 AND kind = 'service' AND value <> '(other)'`, ws).Scan(&used)
	default:
		err = tx.QueryRow(ctx, `SELECT count(*) FROM monitor_names WHERE workspace_id = $1 AND property_id = $2 AND kind = $3 AND value <> '(other)'`,
			ws, prop, kind).Scan(&used)
	}
	if err != nil {
		return nil, err
	}
	var elsewhere map[string]bool
	if kind == KindService {
		rows, err := tx.Query(ctx, `SELECT DISTINCT value FROM monitor_names WHERE workspace_id = $1 AND kind = 'service' AND value = ANY($2)`, ws, missing)
		if err != nil {
			return nil, err
		}
		list, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return nil, err
		}
		elsewhere = map[string]bool{}
		for _, v := range list {
			elsewhere[v] = true
		}
	}
	var next int32
	if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT coalesce(max(id), 0) + 1 FROM %s WHERE workspace_id = $1 AND property_id = $2%s", table, where), args...).Scan(&next); err != nil {
		return nil, err
	}
	limit, capped := Caps[kind]
	var add []string
	var ids []int32
	overflow := false
	for _, v := range missing {
		if special(v) || !capped || used < limit || elsewhere[v] {
			add, ids = append(add, v), append(ids, next)
			got[v] = next
			next++
			if !special(v) && !elsewhere[v] {
				used++
			}
			continue
		}
		overflow = true
	}
	if _, ok := got[OtherValue]; overflow && !ok {
		add, ids = append(add, OtherValue), append(ids, next)
		got[OtherValue] = next
	}
	if len(add) > 0 {
		if kind == KindRoute {
			inf := make([]bool, len(add))
			for i, v := range add {
				inf[i] = inferred[v]
			}
			_, err = tx.Exec(ctx, `INSERT INTO monitor_routes (workspace_id, property_id, id, pattern, inferred, first_seen_at, last_seen_at)
				SELECT $1, $2, u.id, u.pattern, u.inferred, $6, $6 FROM unnest($3::int[], $4::text[], $5::boolean[]) AS u(id, pattern, inferred)`,
				ws, prop, ids, add, inf, now)
		} else {
			_, err = tx.Exec(ctx, `INSERT INTO monitor_names (workspace_id, property_id, kind, id, value, first_seen_at)
				SELECT $1, $2, $3, u.id, u.value, $6 FROM unnest($4::int[], $5::text[]) AS u(id, value)`, ws, prop, kind, ids, add, now)
		}
		if err != nil {
			return nil, fmt.Errorf("monitor names: %w", err)
		}
	}
	for _, v := range missing {
		if _, ok := got[v]; !ok {
			got[v] = got[OtherValue]
		}
	}
	return got, nil
}

// liveProperties are the write's API properties that still exist, in workspaces that are not
// deleted; rows of others are not written.
func liveProperties(ctx context.Context, tx pgx.Tx, w Write) (map[uuid.UUID]bool, error) {
	pairs := map[[2]uuid.UUID]bool{}
	for _, r := range w.Minutes {
		pairs[[2]uuid.UUID{r.WorkspaceID, r.PropertyID}] = true
	}
	for _, r := range w.Dims {
		pairs[[2]uuid.UUID{r.WorkspaceID, r.PropertyID}] = true
	}
	for _, c := range w.Counts {
		pairs[[2]uuid.UUID{c.WorkspaceID, c.PropertyID}] = true
	}
	ws, props := make([]uuid.UUID, 0, len(pairs)), make([]uuid.UUID, 0, len(pairs))
	for p := range pairs {
		ws, props = append(ws, p[0]), append(props, p[1])
	}
	rows, err := tx.Query(ctx, `SELECT p.id FROM analytics_properties p
		JOIN unnest($1::uuid[], $2::uuid[]) AS u(workspace_id, id) ON p.workspace_id = u.workspace_id AND p.id = u.id
		JOIN workspaces w ON w.id = p.workspace_id
		WHERE p.deleted_at IS NULL AND w.deleted_at IS NULL AND p.kind = 'api'`, ws, props)
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

// aggColumns are the columns of an Agg, in the order aggArrays fills them.
const aggColumns = "requests, errors, client_errors, sampled, duration_sum, duration_min, duration_max, durations, request_bytes, response_bytes, response_sizes"

type aggArrays struct {
	requests, errors, clientErrors, sampled, durSum, durMin, durMax, reqBytes, respBytes []int64
	durations, sizes                                                                     [][]byte
}

func (a *aggArrays) add(g *Agg) {
	a.requests, a.errors, a.clientErrors, a.sampled = append(a.requests, g.Requests), append(a.errors, g.Errors), append(a.clientErrors, g.ClientErrors), append(a.sampled, g.Sampled)
	a.durSum, a.durMin, a.durMax = append(a.durSum, g.DurationSum), append(a.durMin, g.DurationMin), append(a.durMax, g.DurationMax)
	a.reqBytes, a.respBytes = append(a.reqBytes, g.RequestBytes), append(a.respBytes, g.ResponseBytes)
	a.durations, a.sizes = append(a.durations, g.Durations.Encode()), append(a.sizes, g.Sizes.Encode())
}

func (a *aggArrays) args() []any {
	return []any{a.requests, a.errors, a.clientErrors, a.sampled, a.durSum, a.durMin, a.durMax, a.durations, a.reqBytes, a.respBytes, a.sizes}
}

const aggTypes = "$%d::bigint[], $%d::bigint[], $%d::bigint[], $%d::bigint[], $%d::bigint[], $%d::bigint[], $%d::bigint[], $%d::bytea[], $%d::bigint[], $%d::bigint[], $%d::bytea[]"

func aggPlaceholders(first int) string {
	n := make([]any, 11)
	for i := range n {
		n[i] = first + i
	}
	return fmt.Sprintf(aggTypes, n...)
}

const aggReplace = `requests = EXCLUDED.requests, errors = EXCLUDED.errors, client_errors = EXCLUDED.client_errors, sampled = EXCLUDED.sampled,
    duration_sum = EXCLUDED.duration_sum, duration_min = EXCLUDED.duration_min, duration_max = EXCLUDED.duration_max, durations = EXCLUDED.durations,
    request_bytes = EXCLUDED.request_bytes, response_bytes = EXCLUDED.response_bytes, response_sizes = EXCLUDED.response_sizes`

func (s *PGStore) Write(ctx context.Context, w Write, now time.Time) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		live, err := liveProperties(ctx, tx, w)
		if err != nil {
			return err
		}
		mins := slices.DeleteFunc(slices.Clone(w.Minutes), func(r MinuteRow) bool { return !live[r.PropertyID] })
		slices.SortFunc(mins, func(a, b MinuteRow) int {
			return cmp.Or(a.PropertyID.Compare(b.PropertyID), a.Minute.Compare(b.Minute), cmp.Compare(a.Writer, b.Writer), compareCore(a.Key, b.Key))
		})
		if len(mins) > 0 {
			n := len(mins)
			ws, props, minute, writer := make([]uuid.UUID, n), make([]uuid.UUID, n), make([]time.Time, n), make([]int64, n)
			env, svc, route, ver := make([]int32, n), make([]int32, n), make([]int32, n), make([]int32, n)
			method, status, source, platform := make([]int16, n), make([]int16, n), make([]int16, n), make([]int16, n)
			var aggs aggArrays
			for i, r := range mins {
				ws[i], props[i], minute[i], writer[i] = r.WorkspaceID, r.PropertyID, r.Minute, r.Writer
				k := r.Key
				env[i], svc[i], route[i], ver[i] = k.Environment, k.Service, k.Route, k.APIVersion
				method[i], status[i], source[i], platform[i] = k.Method, k.Status, int16(k.Source), int16(k.Platform)
				aggs.add(&r.Agg)
			}
			args := append([]any{ws, props, minute, writer, env, svc, route, method, status, ver, source, platform}, aggs.args()...)
			if _, err := tx.Exec(ctx, `INSERT INTO monitor_minute (workspace_id, property_id, minute, writer, environment, service, route, method, status,
    api_version, source, platform, `+aggColumns+`)
SELECT * FROM unnest($1::uuid[], $2::uuid[], $3::timestamptz[], $4::bigint[], $5::int[], $6::int[], $7::int[], $8::smallint[], $9::smallint[],
    $10::int[], $11::smallint[], $12::smallint[], `+aggPlaceholders(13)+`)
ON CONFLICT (property_id, minute, writer, environment, service, route, method, status, api_version, source, platform) DO UPDATE SET `+aggReplace,
				args...); err != nil {
				return fmt.Errorf("monitor minutes: %w", err)
			}
		}
		dims := slices.DeleteFunc(slices.Clone(w.Dims), func(r DimRow) bool { return !live[r.PropertyID] })
		slices.SortFunc(dims, func(a, b DimRow) int {
			return cmp.Or(a.PropertyID.Compare(b.PropertyID), a.Hour.Compare(b.Hour), cmp.Compare(a.Writer, b.Writer), compareDim(a.Key, b.Key))
		})
		if len(dims) > 0 {
			n := len(dims)
			ws, props, hour, writer := make([]uuid.UUID, n), make([]uuid.UUID, n), make([]time.Time, n), make([]int64, n)
			dim, method, class := make([]int16, n), make([]int16, n), make([]int16, n)
			value, route := make([]int32, n), make([]int32, n)
			var aggs aggArrays
			for i, r := range dims {
				ws[i], props[i], hour[i], writer[i] = r.WorkspaceID, r.PropertyID, r.Hour, r.Writer
				dim[i], value[i], route[i], method[i], class[i] = r.Key.Dim, r.Key.Value, r.Key.Route, r.Key.Method, r.Key.StatusClass
				aggs.add(&r.Agg)
			}
			args := append([]any{ws, props, hour, writer, dim, value, route, method, class}, aggs.args()...)
			if _, err := tx.Exec(ctx, `INSERT INTO monitor_dim_hour (workspace_id, property_id, hour, writer, dim, value, route, method, status_class, `+aggColumns+`)
SELECT * FROM unnest($1::uuid[], $2::uuid[], $3::timestamptz[], $4::bigint[], $5::smallint[], $6::int[], $7::int[], $8::smallint[], $9::smallint[],
    `+aggPlaceholders(10)+`)
ON CONFLICT (property_id, hour, writer, dim, value, route, method, status_class) DO UPDATE SET `+aggReplace, args...); err != nil {
				return fmt.Errorf("monitor dimensions: %w", err)
			}
		}
		if err := markHours(ctx, tx, w.Hours, live); err != nil {
			return err
		}
		if err := addCounts(ctx, tx, w.Counts, live); err != nil {
			return err
		}
		if err := addUsage(ctx, tx, monthOf(now), w.Usage); err != nil {
			return err
		}
		return touchRoutes(ctx, tx, mins, now)
	})
}

func compareCore(a, b CoreKey) int {
	return cmp.Or(cmp.Compare(a.Environment, b.Environment), cmp.Compare(a.Service, b.Service), cmp.Compare(a.Route, b.Route),
		cmp.Compare(a.Method, b.Method), cmp.Compare(a.Status, b.Status), cmp.Compare(a.APIVersion, b.APIVersion),
		cmp.Compare(a.Source, b.Source), cmp.Compare(a.Platform, b.Platform))
}

func compareDim(a, b DimKey) int {
	return cmp.Or(cmp.Compare(a.Dim, b.Dim), cmp.Compare(a.Value, b.Value), cmp.Compare(a.Route, b.Route), cmp.Compare(a.Method, b.Method),
		cmp.Compare(a.StatusClass, b.StatusClass))
}

// markHours records the hours that have rows, for the folds, and marks those already folded dirty.
func markHours(ctx context.Context, tx pgx.Tx, hours []HourMark, live map[uuid.UUID]bool) error {
	hours = slices.DeleteFunc(slices.Clone(hours), func(h HourMark) bool { return !live[h.PropertyID] })
	if len(hours) == 0 {
		return nil
	}
	n := len(hours)
	ws, props, hour, day, capped := make([]uuid.UUID, n), make([]uuid.UUID, n), make([]time.Time, n), make([]time.Time, n), make([]bool, n)
	for i, h := range hours {
		ws[i], props[i], hour[i], day[i], capped[i] = h.WorkspaceID, h.PropertyID, h.Hour, h.Day, h.Capped
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO monitor_rollup_hours (workspace_id, property_id, hour, day, capped)
SELECT * FROM unnest($1::uuid[], $2::uuid[], $3::timestamptz[], $4::date[], $5::boolean[])
ON CONFLICT (property_id, hour) DO UPDATE SET
    dirty = monitor_rollup_hours.dirty OR monitor_rollup_hours.built_at IS NOT NULL,
    capped = monitor_rollup_hours.capped OR EXCLUDED.capped`, ws, props, hour, day, capped); err != nil {
		return fmt.Errorf("monitor hours: %w", err)
	}
	return nil
}

func addCounts(ctx context.Context, tx pgx.Tx, counts []Count, live map[uuid.UUID]bool) error {
	counts = slices.DeleteFunc(slices.Clone(counts), func(c Count) bool { return !live[c.PropertyID] || c.N == 0 })
	if len(counts) == 0 {
		return nil
	}
	slices.SortFunc(counts, func(a, b Count) int {
		return cmp.Or(a.PropertyID.Compare(b.PropertyID), a.Day.Compare(b.Day), strings.Compare(a.Reason, b.Reason))
	})
	n := len(counts)
	ws, props, days, reasons, records := make([]uuid.UUID, n), make([]uuid.UUID, n), make([]time.Time, n), make([]string, n), make([]int64, n)
	for i, c := range counts {
		ws[i], props[i], days[i], reasons[i], records[i] = c.WorkspaceID, c.PropertyID, c.Day, c.Reason, c.N
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO monitor_ingest_stats (workspace_id, property_id, day, reason, records)
SELECT u.workspace_id, u.property_id, u.day, u.reason, sum(u.records)
FROM unnest($1::uuid[], $2::uuid[], $3::date[], $4::text[], $5::bigint[]) AS u(workspace_id, property_id, day, reason, records)
GROUP BY u.workspace_id, u.property_id, u.day, u.reason
ORDER BY u.property_id, u.day, u.reason
ON CONFLICT (property_id, day, reason) DO UPDATE SET records = monitor_ingest_stats.records + EXCLUDED.records`,
		ws, props, days, reasons, records); err != nil {
		return fmt.Errorf("monitor ingest stats: %w", err)
	}
	return nil
}

func addUsage(ctx context.Context, tx pgx.Tx, month time.Time, usage map[uuid.UUID]int64) error {
	for _, ws := range slices.SortedFunc(maps.Keys(usage), func(a, b uuid.UUID) int { return a.Compare(b) }) {
		if usage[ws] == 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO usage_counters (workspace_id, month, monitor_requests) SELECT $1, $2, $3 WHERE EXISTS (SELECT 1 FROM workspaces WHERE id = $1)
ON CONFLICT (workspace_id, month) DO UPDATE SET monitor_requests = usage_counters.monitor_requests + EXCLUDED.monitor_requests`,
			ws, month, usage[ws]); err != nil {
			return fmt.Errorf("monitor usage: %w", err)
		}
	}
	return nil
}

func touchRoutes(ctx context.Context, tx pgx.Tx, mins []MinuteRow, now time.Time) error {
	seen := map[[2]uuid.UUID]map[int32]bool{}
	for _, r := range mins {
		k := [2]uuid.UUID{r.WorkspaceID, r.PropertyID}
		if seen[k] == nil {
			seen[k] = map[int32]bool{}
		}
		seen[k][r.Key.Route] = true
	}
	for _, k := range slices.SortedFunc(maps.Keys(seen), func(a, b [2]uuid.UUID) int { return a[1].Compare(b[1]) }) {
		ids := slices.Sorted(maps.Keys(seen[k]))
		if _, err := tx.Exec(ctx, `UPDATE monitor_routes SET last_seen_at = $4
			WHERE workspace_id = $1 AND property_id = $2 AND id = ANY($3) AND last_seen_at < $5`, k[0], k[1], ids, now, now.Add(-time.Minute)); err != nil {
			return fmt.Errorf("monitor routes: %w", err)
		}
	}
	return nil
}
