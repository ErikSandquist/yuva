package analytics

import (
	"context"
	"slices"
	"time"
	"uuid"
)

// HomeDays is how many local days, today included, Home shows per property.
const HomeDays = 7

type HomeProperty struct {
	ID  uuid.UUID
	Loc *time.Location
}

type HomeVisitors struct {
	// Days are the visitors of the last HomeDays local days, oldest first; the last is today.
	Days     []time.Time
	Visitors []int64
	Realtime int64
}

// Home counts each property's visitors per local day from the rollups of days that are built and
// clean (or past the raw window), from the raw visits of the others, and its visitors of the
// realtime window from the raw visits of the days that window touches, in one statement.
func (s *PGStore) Home(ctx context.Context, ws uuid.UUID, props []HomeProperty, rawFrom, now time.Time) (map[uuid.UUID]HomeVisitors, error) {
	out := map[uuid.UUID]HomeVisitors{}
	if len(props) == 0 {
		return out, nil
	}
	since := now.Add(-RealtimeWindow).Truncate(time.Minute).Add(time.Minute)
	var ids []uuid.UUID
	var days []time.Time
	var live []bool
	for _, p := range props {
		today := LocalDay(now, p.Loc)
		first, last := LocalDay(since, p.Loc), today
		h := HomeVisitors{Days: make([]time.Time, HomeDays), Visitors: make([]int64, HomeDays)}
		for i := range HomeDays {
			d := today.AddDate(0, 0, i+1-HomeDays)
			h.Days[i] = d
			ids, days, live = append(ids, p.ID), append(days, d), append(live, !d.Before(first) && !d.After(last))
		}
		out[p.ID] = h
	}
	rows, err := s.Pool.Query(ctx, `WITH want AS (
    SELECT * FROM unnest($2::uuid[], $3::date[], $4::bool[]) AS u(property_id, day, live)
), cls AS (
    SELECT w.property_id, w.day, coalesce(d.built_at IS NOT NULL AND (NOT d.dirty OR w.day < $5), false) AS rolled
    FROM want w
    LEFT JOIN analytics_rollup_days d ON d.workspace_id = $1 AND d.property_id = w.property_id AND d.day = w.day
)
SELECT c.property_id, c.day, CASE
    WHEN c.rolled THEN coalesce((SELECT a.visitors FROM analytics_daily a
        WHERE a.workspace_id = $1 AND a.property_id = c.property_id AND a.day = c.day AND a.dim = $8 AND a.value = ''), 0)
    WHEN c.day >= $5 THEN (SELECT count(DISTINCT v.visitor) FROM analytics_visits v
        WHERE v.workspace_id = $1 AND v.property_id = c.property_id AND v.day = c.day AND NOT v.bot)
    ELSE 0 END::bigint
FROM cls c
UNION ALL
SELECT w.property_id, NULL, count(DISTINCT v.visitor)
FROM want w
JOIN analytics_visits v ON v.workspace_id = $1 AND v.property_id = w.property_id AND v.day = w.day
WHERE w.live AND NOT v.bot AND v.ended_at >= $6 AND v.started_at <= $7
GROUP BY w.property_id
`, ws, ids, days, live, rawFrom, since, now, DimTotal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p uuid.UUID
		var d *time.Time
		var n int64
		if err := rows.Scan(&p, &d, &n); err != nil {
			return nil, err
		}
		h := out[p]
		if d == nil {
			h.Realtime = n
		} else if i := slices.IndexFunc(h.Days, func(day time.Time) bool { return day.Equal(d.UTC()) }); i >= 0 {
			h.Visitors[i] = n
		}
		out[p] = h
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
