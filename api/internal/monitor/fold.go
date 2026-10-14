package monitor

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
)

// aggScan reads the aggColumns of a row.
type aggScan struct {
	agg              Agg
	durations, sizes []byte
}

func (a *aggScan) dest() []any {
	g := &a.agg
	return []any{&g.Requests, &g.Errors, &g.ClientErrors, &g.Sampled, &g.DurationSum, &g.DurationMin, &g.DurationMax, &a.durations,
		&g.RequestBytes, &g.ResponseBytes, &a.sizes}
}

func (a *aggScan) decode() (Agg, error) {
	var err error
	if a.agg.Durations, err = DecodeSketch(a.durations); err != nil {
		return Agg{}, err
	}
	if a.agg.Sizes, err = DecodeSketch(a.sizes); err != nil {
		return Agg{}, err
	}
	return a.agg, nil
}

func aggValues(g *Agg) []any {
	return []any{g.Requests, g.Errors, g.ClientErrors, g.Sampled, g.DurationSum, g.DurationMin, g.DurationMax, g.Durations.Encode(),
		g.RequestBytes, g.ResponseBytes, g.Sizes.Encode()}
}

var aggCopyColumns = []string{"requests", "errors", "client_errors", "sampled", "duration_sum", "duration_min", "duration_max", "durations",
	"request_bytes", "response_bytes", "response_sizes"}

const coreColumns = "environment, service, route, method, status, api_version, source, platform"

var coreCopyColumns = []string{"environment", "service", "route", "method", "status", "api_version", "source", "platform"}

func coreDest(k *CoreKey) []any {
	return []any{&k.Environment, &k.Service, &k.Route, &k.Method, &k.Status, &k.APIVersion, &k.Source, &k.Platform}
}

func coreValues(k CoreKey) []any {
	return []any{k.Environment, k.Service, k.Route, k.Method, k.Status, k.APIVersion, int16(k.Source), int16(k.Platform)}
}

const dimColumns = "dim, value, route, method, status_class"

var dimCopyColumns = []string{"dim", "value", "route", "method", "status_class"}

func dimDest(k *DimKey) []any { return []any{&k.Dim, &k.Value, &k.Route, &k.Method, &k.StatusClass} }

func dimValues(k DimKey) []any { return []any{k.Dim, k.Value, k.Route, k.Method, k.StatusClass} }

func collectCore(rows pgx.Rows) (map[CoreKey]*Agg, error) {
	defer rows.Close()
	out := map[CoreKey]*Agg{}
	for rows.Next() {
		var k CoreKey
		var a aggScan
		if err := rows.Scan(append(coreDest(&k), a.dest()...)...); err != nil {
			return nil, err
		}
		g, err := a.decode()
		if err != nil {
			return nil, err
		}
		if cur := out[k]; cur != nil {
			cur.Merge(&g)
		} else {
			out[k] = &g
		}
	}
	return out, rows.Err()
}

func collectDims(rows pgx.Rows) (map[DimKey]*Agg, error) {
	defer rows.Close()
	out := map[DimKey]*Agg{}
	for rows.Next() {
		var k DimKey
		var a aggScan
		if err := rows.Scan(append(dimDest(&k), a.dest()...)...); err != nil {
			return nil, err
		}
		g, err := a.decode()
		if err != nil {
			return nil, err
		}
		if cur := out[k]; cur != nil {
			cur.Merge(&g)
		} else {
			out[k] = &g
		}
	}
	return out, rows.Err()
}

func sortedCore(m map[CoreKey]*Agg) []CoreKey {
	keys := make([]CoreKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, compareCore)
	return keys
}

func sortedDims(m map[DimKey]*Agg) []DimKey {
	keys := make([]DimKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, compareDim)
	return keys
}

// capDims keeps an hour's top dimension rows by requests and adds the rest up under each
// dimension's (other); it says whether it had to.
func capDims(ctx context.Context, tx pgx.Tx, ws, prop uuid.UUID, rows map[DimKey]*Agg, now time.Time) (bool, error) {
	if len(rows) <= DimRowsPerHour {
		return false, nil
	}
	keys := sortedDims(rows)
	slices.SortStableFunc(keys, func(a, b DimKey) int { return cmp.Compare(rows[b].Requests, rows[a].Requests) })
	other := map[int16]int32{}
	for d, kind := range dimKinds {
		ids, err := names(ctx, tx, ws, prop, kind, []string{OtherValue}, nil, now)
		if err != nil {
			return false, err
		}
		other[d] = ids[OtherValue]
	}
	for _, k := range keys[DimRowsPerHour:] {
		g := rows[k]
		delete(rows, k)
		o := k
		o.Value = other[k.Dim]
		if cur := rows[o]; cur != nil {
			cur.Merge(g)
		} else {
			rows[o] = g
		}
	}
	return true, nil
}

func bumpVersion(ctx context.Context, tx pgx.Tx, ws, prop uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE analytics_properties SET rollup_version = rollup_version + 1 WHERE workspace_id = $1 AND id = $2`, ws, prop)
	return err
}

// FoldHour folds a closed local hour: its minute rows into one hourly row per key, and its
// dimension rows of every writer into one. The hour's row in monitor_rollup_hours is locked first,
// so a writer that adds rows meanwhile marks the hour dirty again rather than being lost.
func (s *PGStore) FoldHour(ctx context.Context, ws, prop uuid.UUID, hour, end time.Time, now time.Time) error {
	if err := s.EnsureHourPartitions(ctx, hour, hour); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var capped bool
		if err := tx.QueryRow(ctx, `SELECT capped FROM monitor_rollup_hours WHERE workspace_id = $1 AND property_id = $2 AND hour = $3 FOR UPDATE`,
			ws, prop, hour).Scan(&capped); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT `+coreColumns+`, `+aggColumns+` FROM monitor_minute
			WHERE workspace_id = $1 AND property_id = $2 AND minute >= $3 AND minute < $4`, ws, prop, hour, end)
		if err != nil {
			return err
		}
		core, err := collectCore(rows)
		if err != nil {
			return fmt.Errorf("fold hour: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM monitor_hour WHERE workspace_id = $1 AND property_id = $2 AND hour = $3`, ws, prop, hour); err != nil {
			return err
		}
		copyRows := make([][]any, 0, len(core))
		for _, k := range sortedCore(core) {
			copyRows = append(copyRows, append(append([]any{ws, prop, hour}, coreValues(k)...), aggValues(core[k])...))
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{HourTable}, append(append([]string{"workspace_id", "property_id", "hour"}, coreCopyColumns...), aggCopyColumns...),
			pgx.CopyFromRows(copyRows)); err != nil {
			return fmt.Errorf("fold hour: %w", err)
		}
		rows, err = tx.Query(ctx, `DELETE FROM monitor_dim_hour WHERE workspace_id = $1 AND property_id = $2 AND hour = $3
			RETURNING `+dimColumns+`, `+aggColumns, ws, prop, hour)
		if err != nil {
			return err
		}
		dims, err := collectDims(rows)
		if err != nil {
			return fmt.Errorf("fold hour: %w", err)
		}
		over, err := capDims(ctx, tx, ws, prop, dims, now)
		if err != nil {
			return err
		}
		copyRows = make([][]any, 0, len(dims))
		for _, k := range sortedDims(dims) {
			copyRows = append(copyRows, append(append([]any{ws, prop, hour, int64(0)}, dimValues(k)...), aggValues(dims[k])...))
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{DimHourTable}, append(append([]string{"workspace_id", "property_id", "hour", "writer"}, dimCopyColumns...), aggCopyColumns...),
			pgx.CopyFromRows(copyRows)); err != nil {
			return fmt.Errorf("fold hour: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE monitor_rollup_hours SET built_at = $4, dirty = false, day_built_at = NULL, capped = capped OR $5
			WHERE workspace_id = $1 AND property_id = $2 AND hour = $3`, ws, prop, hour, now, over); err != nil {
			return err
		}
		return bumpVersion(ctx, tx, ws, prop)
	})
}

// FoldDay folds a finished local day from its hours, once each of them is folded; it reports
// whether it did.
func (s *PGStore) FoldDay(ctx context.Context, ws, prop uuid.UUID, day time.Time, now time.Time) (bool, error) {
	done := false
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT hour, built_at IS NOT NULL AND NOT dirty FROM monitor_rollup_hours
			WHERE workspace_id = $1 AND property_id = $2 AND day = $3 ORDER BY hour FOR UPDATE`, ws, prop, day)
		if err != nil {
			return err
		}
		var hours []time.Time
		ready := true
		for rows.Next() {
			var h time.Time
			var built bool
			if err := rows.Scan(&h, &built); err != nil {
				rows.Close()
				return err
			}
			hours, ready = append(hours, h), ready && built
		}
		rows.Close()
		if err := rows.Err(); err != nil || !ready || len(hours) == 0 {
			return err
		}
		rows, err = tx.Query(ctx, `SELECT `+coreColumns+`, `+aggColumns+` FROM monitor_hour
			WHERE workspace_id = $1 AND property_id = $2 AND hour = ANY($3)`, ws, prop, hours)
		if err != nil {
			return err
		}
		core, err := collectCore(rows)
		if err != nil {
			return fmt.Errorf("fold day: %w", err)
		}
		rows, err = tx.Query(ctx, `SELECT `+dimColumns+`, `+aggColumns+` FROM monitor_dim_hour
			WHERE workspace_id = $1 AND property_id = $2 AND hour = ANY($3)`, ws, prop, hours)
		if err != nil {
			return err
		}
		dims, err := collectDims(rows)
		if err != nil {
			return fmt.Errorf("fold day: %w", err)
		}
		for _, t := range []string{"monitor_day", "monitor_dim_day"} {
			if _, err := tx.Exec(ctx, "DELETE FROM "+t+" WHERE workspace_id = $1 AND property_id = $2 AND day = $3", ws, prop, day); err != nil {
				return err
			}
		}
		copyRows := make([][]any, 0, len(core))
		for _, k := range sortedCore(core) {
			copyRows = append(copyRows, append(append([]any{ws, prop, day}, coreValues(k)...), aggValues(core[k])...))
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"monitor_day"}, append(append([]string{"workspace_id", "property_id", "day"}, coreCopyColumns...), aggCopyColumns...),
			pgx.CopyFromRows(copyRows)); err != nil {
			return fmt.Errorf("fold day: %w", err)
		}
		copyRows = make([][]any, 0, len(dims))
		for _, k := range sortedDims(dims) {
			copyRows = append(copyRows, append(append([]any{ws, prop, day}, dimValues(k)...), aggValues(dims[k])...))
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"monitor_dim_day"}, append(append([]string{"workspace_id", "property_id", "day"}, dimCopyColumns...), aggCopyColumns...),
			pgx.CopyFromRows(copyRows)); err != nil {
			return fmt.Errorf("fold day: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE monitor_rollup_hours SET day_built_at = $4 WHERE workspace_id = $1 AND property_id = $2 AND day = $3`,
			ws, prop, day, now); err != nil {
			return err
		}
		done = true
		return bumpVersion(ctx, tx, ws, prop)
	})
	return done, err
}

// Rollup folds a property's hours that closed FoldAfter ago and are not folded or changed since,
// then its local days that ended FoldAfter ago with every hour folded; it says how many of each.
func (s *PGStore) Rollup(ctx context.Context, ws, prop uuid.UUID, zone Zone, now time.Time) (int, int, error) {
	rows, err := s.Pool.Query(ctx, `SELECT hour FROM monitor_rollup_hours
		WHERE workspace_id = $1 AND property_id = $2 AND (built_at IS NULL OR dirty) AND hour <= $3 ORDER BY hour`,
		ws, prop, now.Add(-time.Hour-FoldAfter))
	if err != nil {
		return 0, 0, err
	}
	hours, err := pgx.CollectRows(rows, pgx.RowTo[time.Time])
	if err != nil {
		return 0, 0, err
	}
	for _, h := range hours {
		if err := s.FoldHour(ctx, ws, prop, h, zone.HourEnd(h), now); err != nil {
			return 0, 0, err
		}
	}
	rows, err = s.Pool.Query(ctx, `SELECT day FROM monitor_rollup_hours WHERE workspace_id = $1 AND property_id = $2
		GROUP BY day HAVING bool_and(built_at IS NOT NULL AND NOT dirty) AND bool_or(day_built_at IS NULL) ORDER BY day`, ws, prop)
	if err != nil {
		return len(hours), 0, err
	}
	days, err := pgx.CollectRows(rows, pgx.RowTo[time.Time])
	if err != nil {
		return len(hours), 0, err
	}
	folded := 0
	for _, d := range days {
		d = d.UTC()
		if now.Before(zone.DayStart(d.AddDate(0, 0, 1)).Add(FoldAfter)) {
			continue
		}
		ok, err := s.FoldDay(ctx, ws, prop, d, now)
		if err != nil {
			return len(hours), folded, err
		}
		if ok {
			folded++
		}
	}
	return len(hours), folded, nil
}
