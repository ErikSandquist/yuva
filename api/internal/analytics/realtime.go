package analytics

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
)

const (
	RealtimeWindow = 30 * time.Minute
	realtimeTop    = 10
)

type TopValue struct {
	Value    string
	Visitors int64
}

type Minute struct {
	At                  time.Time
	Visitors, Pageviews int64
}

type Realtime struct {
	Visitors, Pageviews int64
	Minutes             []Minute
	Pages               []TopValue
	Sources             []TopValue
	Countries           []TopValue
}

// Realtime is the property's last 30 minutes before now, from the raw events of the local days
// that window touches, without bots.
func (s *PGStore) Realtime(ctx context.Context, ws, property uuid.UUID, loc *time.Location, now time.Time) (Realtime, error) {
	since := now.Add(-RealtimeWindow).Truncate(time.Minute).Add(time.Minute)
	days := []time.Time{LocalDay(since, loc)}
	if d := LocalDay(now, loc); !d.Equal(days[0]) {
		days = append(days, d)
	}
	out := Realtime{}
	type ev struct {
		ts                    time.Time
		visitor               int64
		view                  bool
		path, source, country string
	}
	var evs []ev
	err := pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", QueryTimeout.Milliseconds())); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT e.ts, e.visitor, e.kind = 1, e.path, v.source, v.country FROM analytics_events e
JOIN analytics_visits v ON v.workspace_id = e.workspace_id AND v.property_id = e.property_id AND v.day = e.day AND v.visit = e.visit
WHERE e.workspace_id = $1 AND e.property_id = $2 AND e.day = ANY($3::date[]) AND v.day = ANY($3::date[]) AND e.ts >= $4 AND e.ts <= $5 AND NOT v.bot`, ws, property, days, since, now)
		if err != nil {
			return err
		}
		evs, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (ev, error) {
			var e ev
			err := r.Scan(&e.ts, &e.visitor, &e.view, &e.path, &e.source, &e.country)
			return e, err
		})
		return err
	})
	if err != nil {
		return out, err
	}
	type set map[int64]struct{}
	all := set{}
	minutes := map[time.Time]set{}
	views := map[time.Time]int64{}
	tops := []map[string]set{{}, {}, {}}
	for _, e := range evs {
		all[e.visitor] = struct{}{}
		m := e.ts.UTC().Truncate(time.Minute)
		if minutes[m] == nil {
			minutes[m] = set{}
		}
		minutes[m][e.visitor] = struct{}{}
		for i, v := range []string{e.path, e.source, e.country} {
			if i == 0 && !e.view {
				continue
			}
			if tops[i][v] == nil {
				tops[i][v] = set{}
			}
			tops[i][v][e.visitor] = struct{}{}
		}
		if e.view {
			out.Pageviews++
			views[m]++
		}
	}
	out.Visitors = int64(len(all))
	for at := since.UTC(); !at.After(now); at = at.Add(time.Minute) {
		out.Minutes = append(out.Minutes, Minute{At: at, Visitors: int64(len(minutes[at])), Pageviews: views[at]})
	}
	for i, into := range []*[]TopValue{&out.Pages, &out.Sources, &out.Countries} {
		list := []TopValue{}
		for v, vs := range tops[i] {
			list = append(list, TopValue{Value: v, Visitors: int64(len(vs))})
		}
		slices.SortFunc(list, func(a, b TopValue) int {
			return cmp.Or(cmp.Compare(b.Visitors, a.Visitors), strings.Compare(a.Value, b.Value))
		})
		*into = list[:min(len(list), realtimeTop)]
	}
	return out, err
}
