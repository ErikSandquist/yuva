package api

import (
	"context"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	analyticsDeleteBatch = 50_000
	// The delay outlasts the key caches of other instances and one flush, so no event of a deleted
	// property arrives after its rows are gone.
	analyticsDeleteDelay = 2 * time.Minute
)

// AnalyticsDeleteArgs deletes a property's raw rows: of the days From to To, of every day (All,
// with the rollups), or everything and then the property (Property).
type AnalyticsDeleteArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	PropertyID  uuid.UUID `json:"property_id"`
	Property    bool      `json:"property,omitempty"`
	All         bool      `json:"all,omitempty"`
	From        string    `json:"from,omitempty"`
	To          string    `json:"to,omitempty"`
}

func (AnalyticsDeleteArgs) Kind() string { return "analytics_delete" }

func (AnalyticsDeleteArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type analyticsDeleteWorker struct {
	river.WorkerDefaults[AnalyticsDeleteArgs]
	s *Server
}

func (w *analyticsDeleteWorker) Timeout(*river.Job[AnalyticsDeleteArgs]) time.Duration {
	return purgeBudget + 90*time.Second
}

type analyticsPurge struct {
	Rows int64 `json:"rows_deleted"`
	Days int   `json:"days"`
	Done bool  `json:"done"`
}

func (w *analyticsDeleteWorker) Work(ctx context.Context, job *river.Job[AnalyticsDeleteArgs]) error {
	done, err := w.s.DeleteAnalytics(ctx, job.Args, purgeBudget)
	if err != nil {
		return err
	}
	if !done {
		return river.JobSnooze(0)
	}
	return nil
}

// DeleteAnalytics does one run of a deletion for about budget; done reports that nothing is left.
func (s *Server) DeleteAnalytics(ctx context.Context, a AnalyticsDeleteArgs, budget time.Duration) (bool, error) {
	q := s.st.Queries
	if a.Property {
		if _, err := q.GetDeletedAnalyticsProperty(ctx, store.GetDeletedAnalyticsPropertyParams{WorkspaceID: a.WorkspaceID, ID: a.PropertyID}); store.IsNotFound(err) {
			return true, nil
		} else if err != nil {
			return false, err
		}
	} else if _, err := q.GetAnalyticsProperty(ctx, store.GetAnalyticsPropertyParams{WorkspaceID: a.WorkspaceID, ID: a.PropertyID}); store.IsNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}
	var from, to *time.Time
	if a.From != "" {
		f, err := time.Parse(time.DateOnly, a.From)
		if err != nil {
			return false, err
		}
		t, err := time.Parse(time.DateOnly, a.To)
		if err != nil {
			return false, err
		}
		from, to = &f, &t
	}
	p, err := s.purgeAnalyticsRaw(ctx, a.WorkspaceID, a.PropertyID, from, to, time.Now().Add(budget))
	if err != nil {
		return false, err
	}
	if p.Done {
		switch {
		case a.Property:
			_, err = q.DeleteAnalyticsPropertyRow(ctx, store.DeleteAnalyticsPropertyRowParams{WorkspaceID: a.WorkspaceID, ID: a.PropertyID})
		case a.All:
			err = q.DeleteAnalyticsRollups(ctx, store.DeleteAnalyticsRollupsParams{WorkspaceID: a.WorkspaceID, PropertyID: a.PropertyID})
		default:
			_, err = q.MarkAnalyticsDaysDirty(ctx, store.MarkAnalyticsDaysDirtyParams{WorkspaceID: a.WorkspaceID, PropertyID: a.PropertyID, FromDay: *from, ToDay: *to})
		}
		if err != nil {
			return false, err
		}
		if s.analytics != nil {
			s.analytics.forget(a.PropertyID)
		}
	}
	_ = river.RecordOutput(ctx, p)
	s.log.InfoContext(ctx, "analytics deletion", slog.String("workspace_id", a.WorkspaceID.String()), slog.String("property_id", a.PropertyID.String()),
		slog.Int64("rows_deleted", p.Rows), slog.Int("days", p.Days), slog.Bool("done", p.Done))
	return p.Done, nil
}

// purgeAnalyticsRaw deletes a property's visits and events of the days from to (every day when
// nil), partition by partition, until deadline.
func (s *Server) purgeAnalyticsRaw(ctx context.Context, ws, propertyID uuid.UUID, from, to *time.Time, deadline time.Time) (analyticsPurge, error) {
	var out analyticsPurge
	days, err := s.st.ListAnalyticsPropertyDays(ctx, store.ListAnalyticsPropertyDaysParams{WorkspaceID: ws, PropertyID: propertyID, FromDay: from, ToDay: to})
	if err != nil {
		return out, err
	}
	raw := &analytics.PGStore{Pool: s.st.Pool}
	for _, d := range days {
		for {
			n, err := raw.DeleteRaw(ctx, ws, propertyID, d, analyticsDeleteBatch)
			if err != nil {
				return out, err
			}
			out.Rows += n
			if n < analyticsDeleteBatch {
				break
			}
			if time.Now().After(deadline) {
				return out, nil
			}
		}
		out.Days++
		if time.Now().After(deadline) {
			return out, nil
		}
	}
	out.Done = true
	return out, nil
}

// purgeWorkspaceAnalytics deletes the raw rows of every property of a deleted workspace; the rest
// goes with the workspace row.
func (s *Server) purgeWorkspaceAnalytics(ctx context.Context, ws uuid.UUID, deadline time.Time) (bool, error) {
	ids, err := s.st.ListWorkspaceAnalyticsPropertyIDs(ctx, ws)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		p, err := s.purgeAnalyticsRaw(ctx, ws, id, nil, nil, deadline)
		if err != nil || !p.Done {
			return false, err
		}
	}
	return true, nil
}
