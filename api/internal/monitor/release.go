package monitor

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

// DeploysPerDay is how many deploy annotations a property gets in a local day; later new
// releases are still kept, without one.
const DeploysPerDay = 50

// Release is a release a service and environment's records named, at the time of its first record.
type Release struct {
	Service, Environment, Release string
	At                            time.Time
}

func (s *PGStore) NoteReleases(ctx context.Context, p Property, rels []Release, zone Zone, now time.Time) ([]Release, error) {
	if len(rels) == 0 {
		return nil, nil
	}
	rels = slices.Clone(rels)
	slices.SortFunc(rels, func(a, b Release) int {
		return cmp.Or(strings.Compare(a.Service, b.Service), strings.Compare(a.Environment, b.Environment), strings.Compare(a.Release, b.Release))
	})
	n := len(rels)
	services, envs, names, times := make([]string, n), make([]string, n), make([]string, n), make([]time.Time, n)
	for i, r := range rels {
		services[i], envs[i], names[i], times[i] = r.Service, r.Environment, r.Release, r.At
	}
	var out []Release
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `INSERT INTO monitor_releases (workspace_id, property_id, service, environment, release, first_seen_at)
			SELECT $1, $2, u.service, u.environment, u.release, u.at FROM unnest($3::text[], $4::text[], $5::text[], $6::timestamptz[]) AS u(service, environment, release, at)
			WHERE EXISTS (SELECT 1 FROM analytics_properties WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL AND kind = 'api')
			ON CONFLICT (property_id, service, environment, release) DO NOTHING
			RETURNING service, environment, release, first_seen_at`, p.WorkspaceID, p.ID, services, envs, names, times)
		if err != nil {
			return fmt.Errorf("monitor releases: %w", err)
		}
		for rows.Next() {
			var r Release
			if err := rows.Scan(&r.Service, &r.Environment, &r.Release, &r.At); err != nil {
				rows.Close()
				return err
			}
			r.At = r.At.UTC()
			out = append(out, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("monitor releases: %w", err)
		}
		slices.SortFunc(out, func(a, b Release) int { return a.At.Compare(b.At) })
		for _, r := range out {
			day := zone.LocalDay(r.At)
			var made int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM analytics_annotations WHERE workspace_id = $1 AND property_id = $2 AND starts_on = $3 AND kind = 'deploy'`,
				p.WorkspaceID, p.ID, day).Scan(&made); err != nil {
				return err
			}
			if made >= DeploysPerDay {
				continue
			}
			if _, err := tx.Exec(ctx, `INSERT INTO analytics_annotations (id, workspace_id, property_id, kind, starts_on, ends_on, text, at, service, environment, release,
				created_at, updated_at) VALUES ($1, $2, $3, 'deploy', $4, $4, $5, $6, $7, $8, $5, $9, $9)`,
				uuid.NewV7(), p.WorkspaceID, p.ID, day, r.Release, r.At, r.Service, r.Environment, now); err != nil {
				return fmt.Errorf("deploy annotation: %w", err)
			}
		}
		return nil
	})
	return out, err
}
