package monitor

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/productdevbook/yuva/api/internal/analytics"
)

var (
	minutePartition = regexp.MustCompile(`^monitor_minute_(\d{8})$`)
	hourPartition   = regexp.MustCompile(`^monitor_(?:dim_)?hour_(\d{6})$`)
)

// DropPartitionsBefore drops the minute partitions of UTC days before day and the hour partitions
// of months before month. It is server maintenance over every workspace's rows of those days (the
// exception to the workspace rule of analytics_partitions, see architecture.md).
func (s *PGStore) DropPartitionsBefore(ctx context.Context, day, month time.Time) ([]string, error) {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", int64(analytics.PartitionLock)); err != nil {
		return nil, err
	}
	defer conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", int64(analytics.PartitionLock))
	rows, err := conn.Query(ctx, `SELECT c.relname, p.relname, i.inhdetachpending FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname IN ('monitor_minute', 'monitor_hour', 'monitor_dim_hour') ORDER BY c.relname`)
	if err != nil {
		return nil, err
	}
	type part struct {
		name, parent string
		pending      bool
	}
	parts, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (part, error) {
		var p part
		err := r.Scan(&p.name, &p.parent, &p.pending)
		return p, err
	})
	if err != nil {
		return nil, err
	}
	var dropped []string
	for _, p := range parts {
		var old bool
		if m := minutePartition.FindStringSubmatch(p.name); m != nil {
			d, err := time.Parse("20060102", m[1])
			old = err == nil && d.Before(day)
		} else if m := hourPartition.FindStringSubmatch(p.name); m != nil {
			d, err := time.Parse("200601", m[1])
			old = err == nil && d.Before(month)
		}
		if !old {
			continue
		}
		detach := "CONCURRENTLY"
		if p.pending {
			detach = "FINALIZE"
		}
		if _, err := conn.Exec(ctx, fmt.Sprintf("ALTER TABLE %s DETACH PARTITION %s %s", pgx.Identifier{p.parent}.Sanitize(), pgx.Identifier{p.name}.Sanitize(), detach)); err != nil {
			return dropped, fmt.Errorf("detach %s: %w", p.name, err)
		}
		if _, err := conn.Exec(ctx, "DROP TABLE "+pgx.Identifier{p.name}.Sanitize()); err != nil {
			return dropped, fmt.Errorf("drop %s: %w", p.name, err)
		}
		dropped = append(dropped, p.name)
	}
	return dropped, nil
}
