package monitor

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/productdevbook/yuva/api/internal/analytics"
)

const CheckResultsTable = "monitor_check_results"

// ErrRunsRange: a time:minute or time:hour uptime query reaches past the runs kept.
var ErrRunsRange = errors.New("the range reaches past the runs kept")

var (
	UptimeMetrics = []string{"runs", "failures", "up", "uptime", "downtime", "response_time_p50", "response_time_p95", "response_time_p99",
		"avg_response_time", "ttfb", "cert_days_left"}
	UptimeGrains = Grains

	uptimeSketchMetrics = []string{"response_time_p50", "response_time_p95", "response_time_p99"}
	checkPartition      = regexp.MustCompile(`^monitor_check_results_(\d{8})$`)
)

func CheckPartition(day time.Time) string { return CheckResultsTable + "_" + day.Format("20060102") }

func (s *PGStore) EnsureCheckPartitions(ctx context.Context, from, to time.Time) error {
	var parts []partition
	for d := utcDay(from); !d.After(utcDay(to)); d = d.AddDate(0, 0, 1) {
		parts = append(parts, partition{CheckResultsTable, CheckPartition(d), d, d.AddDate(0, 0, 1)})
	}
	return s.ensure(ctx, parts)
}

// DropCheckPartitionsBefore drops the run partitions of UTC days before day: server maintenance
// over every workspace's runs of those days, like the minute partitions.
func (s *PGStore) DropCheckPartitionsBefore(ctx context.Context, day time.Time) ([]string, error) {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", int64(analytics.PartitionLock)); err != nil {
		return nil, err
	}
	defer conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", int64(analytics.PartitionLock))
	rows, err := conn.Query(ctx, `SELECT c.relname, i.inhdetachpending FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname = 'monitor_check_results' ORDER BY c.relname`)
	if err != nil {
		return nil, err
	}
	type part struct {
		name    string
		pending bool
	}
	parts, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (part, error) {
		var p part
		err := r.Scan(&p.name, &p.pending)
		return p, err
	})
	if err != nil {
		return nil, err
	}
	var dropped []string
	for _, p := range parts {
		m := checkPartition.FindStringSubmatch(p.name)
		if m == nil {
			continue
		}
		if d, err := time.Parse("20060102", m[1]); err != nil || !d.Before(day) {
			continue
		}
		detach := "CONCURRENTLY"
		if p.pending {
			detach = "FINALIZE"
		}
		if _, err := conn.Exec(ctx, fmt.Sprintf("ALTER TABLE monitor_check_results DETACH PARTITION %s %s", pgx.Identifier{p.name}.Sanitize(), detach)); err != nil {
			return dropped, fmt.Errorf("detach %s: %w", p.name, err)
		}
		if _, err := conn.Exec(ctx, "DROP TABLE "+pgx.Identifier{p.name}.Sanitize()); err != nil {
			return dropped, fmt.Errorf("drop %s: %w", p.name, err)
		}
		dropped = append(dropped, p.name)
	}
	return dropped, nil
}

type lastRun struct {
	at time.Time
	ok bool
}

type lastCert struct {
	at   time.Time
	days int64
}

// UptimeAgg is what runs add up: counted runs (unknown ones apart), the seconds they stand for,
// the total times of answered runs as a sketch, and each check's last outcome and certificate.
type UptimeAgg struct {
	Runs, Failures, Unknown int64
	Monitored, Down         int64
	Answered                int64
	TotalSum, TTFBSum       int64
	Durations               Sketch
	last                    map[uuid.UUID]lastRun
	cert                    map[uuid.UUID]lastCert
}

func NewUptimeAgg() UptimeAgg {
	return UptimeAgg{Durations: NewSketch(DurationScale), last: map[uuid.UUID]lastRun{}, cert: map[uuid.UUID]lastCert{}}
}

func (a *UptimeAgg) Merge(b *UptimeAgg) {
	a.Runs += b.Runs
	a.Failures += b.Failures
	a.Unknown += b.Unknown
	a.Monitored += b.Monitored
	a.Down += b.Down
	a.Answered += b.Answered
	a.TotalSum += b.TotalSum
	a.TTFBSum += b.TTFBSum
	a.Durations.Merge(b.Durations)
	for id, l := range b.last {
		if cur, ok := a.last[id]; !ok || l.at.After(cur.at) {
			a.last[id] = l
		}
	}
	for id, c := range b.cert {
		if cur, ok := a.cert[id]; !ok || c.at.After(cur.at) {
			a.cert[id] = c
		}
	}
}

// Metric is an uptime metric of a's runs.
func (a *UptimeAgg) Metric(name string) float64 {
	ms := func(us float64) float64 { return round(us/1000, 3) }
	switch name {
	case "runs":
		return float64(a.Runs)
	case "failures":
		return float64(a.Failures)
	case "up":
		n := 0
		for _, l := range a.last {
			if l.ok {
				n++
			}
		}
		return float64(n)
	case "uptime":
		if a.Monitored == 0 {
			return 0
		}
		return round(float64(a.Monitored-a.Down)*100/float64(a.Monitored), 3)
	case "downtime":
		return float64(a.Down)
	case "response_time_p50", "response_time_p95", "response_time_p99":
		q := map[string]float64{"response_time_p50": 0.5, "response_time_p95": 0.95, "response_time_p99": 0.99}[name]
		if a.Durations.Count() == 0 {
			return 0
		}
		return ms(a.Durations.Quantile(q))
	case "avg_response_time":
		if a.Answered == 0 {
			return 0
		}
		return ms(float64(a.TotalSum) / float64(a.Answered))
	case "ttfb":
		if a.Answered == 0 {
			return 0
		}
		return ms(float64(a.TTFBSum) / float64(a.Answered))
	case "cert_days_left":
		first, out := true, int64(0)
		for _, c := range a.cert {
			if first || c.days < out {
				out, first = c.days, false
			}
		}
		return float64(out)
	}
	return 0
}

// UptimeCheck is a check an uptime query reads, with the last UTC day folded into daily rows.
type UptimeCheck struct {
	ID          uuid.UUID
	FoldedUntil *time.Time
}

// UptimeQuery asks for the runs of Checks in [Start, End), by up to a check and a UTC time grain.
// ResultsFrom is the first instant runs are kept for.
type UptimeQuery struct {
	WorkspaceID uuid.UUID
	Checks      []UptimeCheck
	Start, End  time.Time
	Metrics     []string
	Dimensions  []string
	Now         time.Time
	ResultsFrom time.Time
}

type UptimeRow struct {
	Start time.Time
	Dims  []string
	Agg   UptimeAgg
}

type UptimeResult struct {
	Rows       []UptimeRow
	Totals     UptimeAgg
	ServedFrom string
}

type uptimePlan struct {
	q       UptimeQuery
	grain   string
	byCheck bool
	sketch  bool
}

func (p *uptimePlan) parse() error {
	q := p.q
	if !q.End.After(q.Start) {
		return badQuery("the range ends before it starts")
	}
	if len(q.Dimensions) > 2 {
		return badQuery("at most two dimensions")
	}
	for _, d := range q.Dimensions {
		switch {
		case slices.Contains(Grains, d) && p.grain == "":
			p.grain = strings.TrimPrefix(d, "time:")
		case d == "check" && !p.byCheck:
			p.byCheck = true
		default:
			return badQuery("the uptime dataset breaks down by check and one of %s, each once, not %q", strings.Join(Grains, ", "), d)
		}
	}
	for _, m := range q.Metrics {
		if !slices.Contains(UptimeMetrics, m) {
			return badQuery("the uptime dataset offers %s, not %s", strings.Join(UptimeMetrics, ", "), m)
		}
		p.sketch = p.sketch || slices.Contains(uptimeSketchMetrics, m)
	}
	if (p.grain == "minute" || p.grain == "hour") && q.Start.Before(q.ResultsFrom) {
		return fmt.Errorf("%w: time:%s reads the runs, which are kept from %s", ErrRunsRange, p.grain, q.ResultsFrom.Format(time.RFC3339))
	}
	if p.grain == "minute" && q.End.Sub(q.Start) > MaxMinuteBuckets*time.Minute {
		return ErrMinuteRange
	}
	return nil
}

func (p *uptimePlan) daily() bool { return p.grain == "day" || p.grain == "week" || p.grain == "month" }

func (p *uptimePlan) bucketSQL(col string) string {
	switch p.grain {
	case "minute":
		return "date_trunc('minute', " + col + " AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	case "hour":
		return "date_trunc('hour', " + col + " AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
	case "":
		return "'epoch'::timestamptz"
	}
	return "date_trunc('day', " + col + " AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'"
}

func (p *uptimePlan) coarse(t time.Time) time.Time {
	t = t.UTC()
	switch p.grain {
	case "week":
		return utcDay(t).AddDate(0, 0, -((int(t.Weekday()) + 6) % 7))
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	case "":
		return time.Time{}
	}
	return t
}

func (p *uptimePlan) step(t time.Time) time.Time {
	switch p.grain {
	case "minute":
		return t.Add(time.Minute)
	case "hour":
		return t.Add(time.Hour)
	case "week":
		return t.AddDate(0, 0, 7)
	case "month":
		return t.AddDate(0, 1, 0)
	}
	return t.AddDate(0, 0, 1)
}

func (p *uptimePlan) label(t time.Time) string {
	if p.daily() {
		return t.UTC().Format(time.DateOnly)
	}
	return t.UTC().Format(minuteLabel)
}

// fullDays is the UTC days wholly inside the range, which daily rows may answer.
func (p *uptimePlan) fullDays() (time.Time, time.Time) {
	if p.grain == "minute" || p.grain == "hour" {
		return time.Time{}, time.Time{}
	}
	first := utcDay(p.q.Start)
	if first.Before(p.q.Start) {
		first = first.AddDate(0, 0, 1)
	}
	end := utcDay(p.q.End)
	if !end.After(first) {
		return time.Time{}, time.Time{}
	}
	return first, end
}

type uptimeGroup struct {
	bucket time.Time
	check  uuid.UUID
	agg    UptimeAgg
}

const uptimeSums = `count(*) FILTER (WHERE r.state <> 'unknown'), count(*) FILTER (WHERE r.state <> 'unknown' AND NOT r.ok),
	count(*) FILTER (WHERE r.state = 'unknown'), coalesce(sum(r.seconds) FILTER (WHERE r.state <> 'unknown'), 0)::bigint,
	coalesce(sum(r.seconds) FILTER (WHERE r.state = 'down'), 0)::bigint, count(*) FILTER (WHERE r.state <> 'unknown' AND r.answered),
	coalesce(sum(r.total_us) FILTER (WHERE r.state <> 'unknown' AND r.answered), 0)::bigint,
	coalesce(sum(r.ttfb_us) FILTER (WHERE r.state <> 'unknown' AND r.answered), 0)::bigint,
	max(r.at) FILTER (WHERE r.state <> 'unknown'), (array_agg(r.ok ORDER BY r.at DESC) FILTER (WHERE r.state <> 'unknown'))[1],
	max(r.at) FILTER (WHERE r.cert_days_left IS NOT NULL), (array_agg(r.cert_days_left ORDER BY r.at DESC) FILTER (WHERE r.cert_days_left IS NOT NULL))[1]`

func scanUptime(rows pgx.Rows, sketch, fromDays bool) ([]uptimeGroup, error) {
	defer rows.Close()
	var out []uptimeGroup
	for rows.Next() {
		g := uptimeGroup{agg: NewUptimeAgg()}
		a := &g.agg
		var lastAt, certAt *time.Time
		var lastOK *bool
		var cert *int64
		var totals []int64
		var sketches [][]byte
		dest := []any{&g.check, &g.bucket, &a.Runs, &a.Failures, &a.Unknown, &a.Monitored, &a.Down, &a.Answered, &a.TotalSum, &a.TTFBSum,
			&lastAt, &lastOK, &certAt, &cert}
		if sketch && fromDays {
			dest = append(dest, &sketches)
		} else if sketch {
			dest = append(dest, &totals)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		for _, t := range totals {
			a.Durations.Add(t, 1)
		}
		for _, raw := range sketches {
			sk, err := DecodeSketch(raw)
			if err != nil {
				return nil, err
			}
			a.Durations.Merge(sk)
		}
		if lastAt != nil && lastOK != nil {
			a.last[g.check] = lastRun{at: *lastAt, ok: *lastOK}
		}
		if certAt != nil && cert != nil {
			a.cert[g.check] = lastCert{at: *certAt, days: *cert}
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// readUptime adds up the runs of the plan's checks by check and bucket: whole UTC days a check
// folded from its daily rows, the rest from its runs.
func (p *uptimePlan) read(ctx context.Context, tx pgx.Tx) ([]uptimeGroup, bool, bool, error) {
	q := p.q
	ids := make([]uuid.UUID, len(q.Checks))
	folded := make([]*time.Time, len(q.Checks))
	for i, c := range q.Checks {
		ids[i], folded[i] = c.ID, c.FoldedUntil
	}
	first, end := p.fullDays()
	var out []uptimeGroup
	usedDays, usedRuns := false, false
	if !first.IsZero() {
		sel := `SELECT r.check_id, ` + p.bucketSQL("(r.day::timestamp AT TIME ZONE 'UTC')") + `, sum(r.runs)::bigint, sum(r.failures)::bigint, sum(r.unknown)::bigint,
			sum(r.monitored_seconds)::bigint, sum(r.down_seconds)::bigint, sum(r.answered)::bigint, sum(r.total_sum_us)::bigint, sum(r.ttfb_sum_us)::bigint,
			max(r.last_at), (array_agg(r.last_ok ORDER BY r.last_at DESC NULLS LAST))[1], max(r.cert_at), (array_agg(r.cert_days_left::bigint ORDER BY r.cert_at DESC NULLS LAST))[1]`
		if p.sketch {
			sel += `, array_agg(r.durations)`
		}
		rows, err := tx.Query(ctx, sel+` FROM monitor_check_days r JOIN unnest($2::uuid[], $3::date[]) AS c(id, folded) ON r.check_id = c.id
			WHERE r.workspace_id = $1 AND r.day >= $4 AND r.day < $5 AND r.day <= c.folded GROUP BY 1, 2`, q.WorkspaceID, ids, folded, first, end)
		if err != nil {
			return nil, false, false, fmt.Errorf("uptime days: %w", err)
		}
		gs, err := scanUptime(rows, p.sketch, true)
		if err != nil {
			return nil, false, false, fmt.Errorf("uptime days: %w", err)
		}
		usedDays = len(gs) > 0
		out = append(out, gs...)
	}
	sel := `SELECT r.check_id, ` + p.bucketSQL("r.at") + `, ` + uptimeSums
	if p.sketch {
		sel += `, coalesce(array_agg(r.total_us) FILTER (WHERE r.state <> 'unknown' AND r.answered), '{}')`
	}
	args := []any{q.WorkspaceID, ids, folded, q.Start, q.End}
	exclude := ""
	if !first.IsZero() {
		args = append(args, first, end)
		exclude = ` AND NOT (c.folded IS NOT NULL AND r.at >= $6 AND r.at < least($7::date, c.folded + 1)::timestamp AT TIME ZONE 'UTC')`
	}
	rows, err := tx.Query(ctx, sel+` FROM monitor_check_results r JOIN unnest($2::uuid[], $3::date[]) AS c(id, folded) ON r.check_id = c.id
		WHERE r.workspace_id = $1 AND r.at >= $4 AND r.at < $5 AND r.state IS NOT NULL`+exclude+` GROUP BY 1, 2`, args...)
	if err != nil {
		return nil, false, false, fmt.Errorf("uptime runs: %w", err)
	}
	gs, err := scanUptime(rows, p.sketch, false)
	if err != nil {
		return nil, false, false, fmt.Errorf("uptime runs: %w", err)
	}
	usedRuns = len(gs) > 0
	return append(out, gs...), usedDays, usedRuns, nil
}

func (s *PGStore) QueryUptime(ctx context.Context, q UptimeQuery) (UptimeResult, error) {
	p := &uptimePlan{q: q}
	if err := p.parse(); err != nil {
		return UptimeResult{}, err
	}
	var groups []uptimeGroup
	var days, runs bool
	if len(q.Checks) > 0 {
		err := readOnly(ctx, s, func(tx pgx.Tx) error {
			var err error
			groups, days, runs, err = p.read(ctx, tx)
			return err
		})
		if err != nil {
			return UptimeResult{}, err
		}
	}
	res := p.result(groups)
	switch {
	case runs && days:
		res.ServedFrom = ServedMixed
	case runs || !days:
		res.ServedFrom = ServedRaw
	default:
		res.ServedFrom = ServedRollups
	}
	return res, nil
}

func (p *uptimePlan) result(groups []uptimeGroup) UptimeResult {
	q := p.q
	res := UptimeResult{Totals: NewUptimeAgg()}
	type key struct {
		bucket int64
		check  uuid.UUID
	}
	merged := map[key]*UptimeRow{}
	var order []key
	for _, g := range groups {
		b := p.coarse(g.bucket)
		k := key{bucket: b.UnixNano()}
		if p.byCheck {
			k.check = g.check
		}
		row := merged[k]
		if row == nil {
			row = &UptimeRow{Start: b, Agg: NewUptimeAgg()}
			if p.grain != "" {
				row.Dims = append(row.Dims, p.label(b))
			}
			if p.byCheck {
				row.Dims = append(row.Dims, g.check.String())
			}
			merged[k] = row
			order = append(order, k)
		}
		row.Agg.Merge(&g.agg)
		res.Totals.Merge(&g.agg)
	}
	if p.grain != "" && !p.byCheck {
		for t := p.coarse(p.firstBucket()); t.Before(q.End); t = p.step(t) {
			k := key{bucket: t.UnixNano()}
			if merged[k] == nil {
				merged[k] = &UptimeRow{Start: t, Dims: []string{p.label(t)}, Agg: NewUptimeAgg()}
				order = append(order, k)
			}
		}
	}
	if p.grain == "" && !p.byCheck {
		return res
	}
	for _, k := range order {
		res.Rows = append(res.Rows, *merged[k])
	}
	if p.grain != "" {
		slices.SortStableFunc(res.Rows, func(a, b UptimeRow) int { return a.Start.Compare(b.Start) })
	}
	if len(q.Dimensions) > 1 && q.Dimensions[0] == "check" {
		for i := range res.Rows {
			slices.Reverse(res.Rows[i].Dims)
		}
	}
	return res
}

func (p *uptimePlan) firstBucket() time.Time {
	switch p.grain {
	case "minute":
		return p.q.Start.UTC().Truncate(time.Minute)
	case "hour":
		return p.q.Start.UTC().Truncate(time.Hour)
	}
	return utcDay(p.q.Start)
}

// FoldCheckDay folds a check's runs of a UTC day into its daily row and marks the day folded.
func (s *PGStore) FoldCheckDay(ctx context.Context, ws, check uuid.UUID, day time.Time) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT r.check_id, 'epoch'::timestamptz, `+uptimeSums+`,
			coalesce(array_agg(r.total_us) FILTER (WHERE r.state <> 'unknown' AND r.answered), '{}')
			FROM monitor_check_results r WHERE r.workspace_id = $1 AND r.check_id = $2 AND r.at >= $3 AND r.at < $4 AND r.state IS NOT NULL
			GROUP BY 1`, ws, check, day, day.AddDate(0, 0, 1))
		if err != nil {
			return err
		}
		gs, err := scanUptime(rows, true, false)
		if err != nil {
			return err
		}
		if len(gs) > 0 {
			a := gs[0].agg
			var lastAt, certAt *time.Time
			var lastOK *bool
			var cert *int32
			if l, ok := a.last[check]; ok {
				lastAt, lastOK = &l.at, &l.ok
			}
			if c, ok := a.cert[check]; ok {
				certAt, cert = &c.at, new(int32(c.days))
			}
			if _, err := tx.Exec(ctx, `INSERT INTO monitor_check_days (workspace_id, check_id, day, runs, failures, unknown, monitored_seconds, down_seconds,
				answered, total_sum_us, ttfb_sum_us, durations, last_at, last_ok, cert_at, cert_days_left)
				SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
				WHERE EXISTS (SELECT 1 FROM monitor_checks WHERE workspace_id = $1 AND id = $2)
				ON CONFLICT (check_id, day) DO UPDATE SET runs = EXCLUDED.runs, failures = EXCLUDED.failures, unknown = EXCLUDED.unknown,
				monitored_seconds = EXCLUDED.monitored_seconds, down_seconds = EXCLUDED.down_seconds, answered = EXCLUDED.answered,
				total_sum_us = EXCLUDED.total_sum_us, ttfb_sum_us = EXCLUDED.ttfb_sum_us, durations = EXCLUDED.durations, last_at = EXCLUDED.last_at,
				last_ok = EXCLUDED.last_ok, cert_at = EXCLUDED.cert_at, cert_days_left = EXCLUDED.cert_days_left`,
				ws, check, day, a.Runs, a.Failures, a.Unknown, a.Monitored, a.Down, a.Answered, a.TotalSum, a.TTFBSum, a.Durations.Encode(),
				lastAt, lastOK, certAt, cert); err != nil {
				return fmt.Errorf("check day: %w", err)
			}
		}
		_, err = tx.Exec(ctx, `UPDATE monitor_checks SET folded_until = $3 WHERE workspace_id = $1 AND id = $2 AND (folded_until IS NULL OR folded_until < $3)`,
			ws, check, day)
		return err
	})
}
