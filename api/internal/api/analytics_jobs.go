package api

import (
	"context"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/productdevbook/yuva/api/internal/analytics"
	"github.com/productdevbook/yuva/api/internal/jobs"
	"github.com/productdevbook/yuva/api/internal/realtime"
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
			if err = s.deletePropertyExports(ctx, a.WorkspaceID, a.PropertyID); err != nil {
				return false, err
			}
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
		if err := s.deletePropertyExports(ctx, ws, id); err != nil {
			return false, err
		}
		p, err := s.purgeAnalyticsRaw(ctx, ws, id, nil, nil, deadline)
		if err != nil || !p.Done {
			return false, err
		}
	}
	return true, nil
}

const (
	analyticsRollupTimeout = 5 * time.Minute
	analyticsStatsDays     = 90
	partitionsBehind       = 4
	partitionsAhead        = 7
	// A day's drops reach the operator digest when they are more than half of its events and at
	// least this many, so a quiet day's few refusals do not.
	analyticsDropNoticeMin = 100
	noticeAnalyticsDropped = "analytics_dropped"
	geoipUpdateTimeout     = 15 * time.Minute
	analyticsGeoIPNotice   = analyticsInvalidation + "geoip"
)

var analyticsRollupStates = []rivertype.JobState{
	rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled, rivertype.JobStateRetryable,
}

// AnalyticsRollupArgs rolls up one local day of a property.
type AnalyticsRollupArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	PropertyID  uuid.UUID `json:"property_id"`
	Day         string    `json:"day"`
}

func (AnalyticsRollupArgs) Kind() string { return "analytics_rollup" }

func (AnalyticsRollupArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.AnalyticsQueue, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: analyticsRollupStates}}
}

type analyticsRollupWorker struct {
	river.WorkerDefaults[AnalyticsRollupArgs]
	s *Server
}

func (w *analyticsRollupWorker) Timeout(*river.Job[AnalyticsRollupArgs]) time.Duration {
	return analyticsRollupTimeout
}

func (w *analyticsRollupWorker) Work(ctx context.Context, job *river.Job[AnalyticsRollupArgs]) error {
	day, err := time.Parse(time.DateOnly, job.Args.Day)
	if err != nil {
		return err
	}
	return w.s.RollupAnalyticsDay(ctx, job.Args.WorkspaceID, job.Args.PropertyID, day)
}

// AnalyticsGoalRollupArgs rebuilds the goal and prop rollups of one rolled-up local day of a
// property after its goals or tracked props changed.
type AnalyticsGoalRollupArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	PropertyID  uuid.UUID `json:"property_id"`
	Day         string    `json:"day"`
}

func (AnalyticsGoalRollupArgs) Kind() string { return "analytics_goal_rollup" }

func (AnalyticsGoalRollupArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.AnalyticsQueue, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: analyticsRollupStates}}
}

type analyticsGoalRollupWorker struct {
	river.WorkerDefaults[AnalyticsGoalRollupArgs]
	s *Server
}

func (w *analyticsGoalRollupWorker) Timeout(*river.Job[AnalyticsGoalRollupArgs]) time.Duration {
	return analyticsRollupTimeout
}

func (w *analyticsGoalRollupWorker) Work(ctx context.Context, job *river.Job[AnalyticsGoalRollupArgs]) error {
	day, err := time.Parse(time.DateOnly, job.Args.Day)
	if err != nil {
		return err
	}
	return w.s.RollupAnalyticsGoals(ctx, job.Args.WorkspaceID, job.Args.PropertyID, day)
}

// RollupAnalyticsGoals rebuilds a rolled-up day's goal and prop rollups.
func (s *Server) RollupAnalyticsGoals(ctx context.Context, ws, propertyID uuid.UUID, day time.Time) error {
	if s.analytics == nil {
		return nil
	}
	pg := &analytics.PGStore{Pool: s.st.Pool}
	return pg.RollupGoalsDay(ctx, ws, propertyID, day, s.analytics.settings.RollupTop)
}

// AnalyticsFoldArgs folds the visitors of an open local day of a property whose visits are over.
type AnalyticsFoldArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	PropertyID  uuid.UUID `json:"property_id"`
	Day         string    `json:"day"`
}

func (AnalyticsFoldArgs) Kind() string { return "analytics_fold" }

func (AnalyticsFoldArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.AnalyticsQueue, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: analyticsRollupStates}}
}

type analyticsFoldWorker struct {
	river.WorkerDefaults[AnalyticsFoldArgs]
	s *Server
}

func (w *analyticsFoldWorker) Timeout(*river.Job[AnalyticsFoldArgs]) time.Duration {
	return analyticsRollupTimeout
}

func (w *analyticsFoldWorker) Work(ctx context.Context, job *river.Job[AnalyticsFoldArgs]) error {
	day, err := time.Parse(time.DateOnly, job.Args.Day)
	if err != nil {
		return err
	}
	return w.s.FoldAnalyticsDay(ctx, job.Args.WorkspaceID, job.Args.PropertyID, day)
}

type analyticsPartitionsWorker struct {
	river.WorkerDefaults[jobs.AnalyticsPartitionsArgs]
	s *Server
}

func (w *analyticsPartitionsWorker) Work(ctx context.Context, _ *river.Job[jobs.AnalyticsPartitionsArgs]) error {
	return w.s.MaintainAnalytics(ctx)
}

type analyticsRollupScanWorker struct {
	river.WorkerDefaults[jobs.AnalyticsRollupScanArgs]
	s *Server
}

func (w *analyticsRollupScanWorker) Work(ctx context.Context, _ *river.Job[jobs.AnalyticsRollupScanArgs]) error {
	_, err := w.s.ScanAnalyticsRollups(ctx)
	return err
}

type analyticsSaltsWorker struct {
	river.WorkerDefaults[jobs.AnalyticsSaltsArgs]
	s *Server
}

func (w *analyticsSaltsWorker) Work(ctx context.Context, _ *river.Job[jobs.AnalyticsSaltsArgs]) error {
	_, err := w.s.DeleteExpiredAnalyticsSalts(ctx)
	return err
}

type analyticsGeoIPWorker struct {
	river.WorkerDefaults[jobs.AnalyticsGeoIPArgs]
	s *Server
}

// Work asks every process to fetch the month's database; each downloads it only if its own
// directory does not have it yet.
func (w *analyticsGeoIPWorker) Work(ctx context.Context, _ *river.Job[jobs.AnalyticsGeoIPArgs]) error {
	if w.s.analytics == nil || w.s.analytics.geo == nil {
		return nil
	}
	return w.s.st.NotifySignal(ctx, store.NotifySignalParams{Channel: realtime.InvalidateChannel, Payload: analyticsGeoIPNotice})
}

func (s *Server) addAnalyticsWorkers(workers *river.Workers) {
	river.AddWorker(workers, &analyticsDeleteWorker{s: s})
	river.AddWorker(workers, &analyticsRollupWorker{s: s})
	river.AddWorker(workers, &analyticsGoalRollupWorker{s: s})
	river.AddWorker(workers, &analyticsFoldWorker{s: s})
	river.AddWorker(workers, &analyticsPartitionsWorker{s: s})
	river.AddWorker(workers, &analyticsRollupScanWorker{s: s})
	river.AddWorker(workers, &analyticsSaltsWorker{s: s})
	river.AddWorker(workers, &analyticsGeoIPWorker{s: s})
	river.AddWorker(workers, &analyticsExportWorker{s: s})
}

// rawFrom is the oldest day whose raw rows the partitions job keeps.
func (s *Server) rawFrom() time.Time {
	return analytics.LocalDay(s.now(), time.UTC).AddDate(0, 0, 1-s.analytics.settings.RawDays)
}

// MaintainAnalytics keeps raw partitions from four days back to a week ahead, drops those past the
// raw window, and deletes rollups past each property's retention and ingest counts past 90 days.
func (s *Server) MaintainAnalytics(ctx context.Context) error {
	if s.analytics == nil {
		return nil
	}
	today := analytics.LocalDay(s.now(), time.UTC)
	if err := s.analytics.store.EnsurePartitions(ctx, today.AddDate(0, 0, -partitionsBehind), today.AddDate(0, 0, partitionsAhead)); err != nil {
		return err
	}
	pg := &analytics.PGStore{Pool: s.st.Pool}
	dropped, err := pg.DropPartitionsBefore(ctx, s.rawFrom().AddDate(0, 0, -1))
	if err != nil {
		return err
	}
	if len(dropped) > 0 {
		s.log.InfoContext(ctx, "analytics partitions dropped", slog.Int("count", len(dropped)))
	}
	if n, err := s.CleanAnalyticsExports(ctx); err != nil {
		return err
	} else if n > 0 {
		s.log.InfoContext(ctx, "analytics exports expired", slog.Int("count", n))
	}
	ids, err := s.st.ListAnalyticsWorkspaceIDs(ctx)
	if err != nil {
		return err
	}
	for _, ws := range ids {
		props, err := s.st.ListAnalyticsRetention(ctx, ws)
		if err != nil {
			return err
		}
		for _, p := range props {
			before := analytics.LocalDay(s.now(), propertyLocation(p.Timezone)).AddDate(0, -int(*p.RetentionMonths), 0)
			arg := store.TrimAnalyticsDailyParams{WorkspaceID: ws, PropertyID: p.ID, Before: before}
			for _, trim := range []func(context.Context, store.TrimAnalyticsDailyParams) (int64, error){
				s.st.TrimAnalyticsDaily,
				func(ctx context.Context, a store.TrimAnalyticsDailyParams) (int64, error) {
					return s.st.TrimAnalyticsHourly(ctx, store.TrimAnalyticsHourlyParams(a))
				},
				func(ctx context.Context, a store.TrimAnalyticsDailyParams) (int64, error) {
					return s.st.TrimAnalyticsDailyGoals(ctx, store.TrimAnalyticsDailyGoalsParams(a))
				},
				func(ctx context.Context, a store.TrimAnalyticsDailyParams) (int64, error) {
					return s.st.TrimAnalyticsDailyProps(ctx, store.TrimAnalyticsDailyPropsParams(a))
				},
				func(ctx context.Context, a store.TrimAnalyticsDailyParams) (int64, error) {
					return s.st.TrimAnalyticsRollupDays(ctx, store.TrimAnalyticsRollupDaysParams(a))
				},
			} {
				for {
					n, err := trim(ctx, arg)
					if err != nil {
						return err
					}
					if n < 1000 {
						break
					}
				}
			}
		}
		if _, err := s.st.DeleteOldAnalyticsIngestStats(ctx, store.DeleteOldAnalyticsIngestStatsParams{WorkspaceID: ws, Before: today.AddDate(0, 0, -analyticsStatsDays)}); err != nil {
			return err
		}
	}
	return nil
}

// ScanAnalyticsRollups queues a rollup for every day that ended more than the salt grace ago
// without one, and for every dirty day, and a fold for every open day; it returns how many
// rollups it queued.
func (s *Server) ScanAnalyticsRollups(ctx context.Context) (int, error) {
	if s.analytics == nil {
		return 0, nil
	}
	ids, err := s.st.ListAnalyticsWorkspaceIDs(ctx)
	if err != nil {
		return 0, err
	}
	now, queued := s.now(), 0
	for _, ws := range ids {
		todo, err := s.st.ListAnalyticsRollupTodo(ctx, store.ListAnalyticsRollupTodoParams{WorkspaceID: ws, Oldest: s.rawFrom()})
		if err != nil {
			return queued, err
		}
		var batch, folds []river.InsertManyParams
		for _, t := range todo {
			if now.Before(analytics.DayEnd(t.Day, propertyLocation(t.Timezone)).Add(s.analytics.settings.SaltGrace)) {
				if t.BuiltAt == nil {
					folds = append(folds, river.InsertManyParams{Args: AnalyticsFoldArgs{WorkspaceID: ws, PropertyID: t.PropertyID, Day: t.Day.Format(time.DateOnly)}})
				}
				continue
			}
			batch = append(batch, river.InsertManyParams{Args: AnalyticsRollupArgs{WorkspaceID: ws, PropertyID: t.PropertyID, Day: t.Day.Format(time.DateOnly)}})
		}
		if len(batch) > 0 {
			if _, err := s.jobs.InsertMany(ctx, batch); err != nil {
				return queued, err
			}
			queued += len(batch)
		}
		if len(folds) > 0 {
			if _, err := s.jobs.InsertMany(ctx, folds); err != nil {
				return queued, err
			}
		}
		stale, err := s.st.ListAnalyticsGoalRollupTodo(ctx, store.ListAnalyticsGoalRollupTodoParams{WorkspaceID: ws, Oldest: s.rawFrom()})
		if err != nil {
			return queued, err
		}
		var goals []river.InsertManyParams
		for _, t := range stale {
			goals = append(goals, river.InsertManyParams{Args: AnalyticsGoalRollupArgs{WorkspaceID: ws, PropertyID: t.PropertyID, Day: t.Day.Format(time.DateOnly)}})
		}
		if len(goals) > 0 {
			if _, err := s.jobs.InsertMany(ctx, goals); err != nil {
				return queued, err
			}
			queued += len(goals)
		}
	}
	return queued, nil
}

// RollupAnalyticsDay builds a property's rollups of one local day. The first time a day is built,
// a day whose events were mostly dropped goes into the operator digest.
func (s *Server) RollupAnalyticsDay(ctx context.Context, ws, propertyID uuid.UUID, day time.Time) error {
	if s.analytics == nil {
		return nil
	}
	prop, err := s.st.GetAnalyticsProperty(ctx, store.GetAnalyticsPropertyParams{WorkspaceID: ws, ID: propertyID})
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	pg := &analytics.PGStore{Pool: s.st.Pool}
	res, err := pg.RollupDay(ctx, ws, propertyID, day, prop.Timezone, s.analytics.settings.RollupTop)
	if err != nil {
		return err
	}
	if !res.First || len(s.operator) == 0 || res.Dropped < analyticsDropNoticeMin || res.Dropped <= res.Accepted {
		return nil
	}
	w, err := s.st.GetWorkspace(ctx, ws)
	if err != nil {
		return err
	}
	return s.st.CreateOperatorNotice(ctx, store.CreateOperatorNoticeParams{
		ID: uuid.New(), Kind: noticeAnalyticsDropped, WorkspaceName: &w.Name, CreatedAt: s.now(),
		Detail: new(formatCount(res.Dropped) + " of " + formatCount(res.Dropped+res.Accepted) + " analytics events on " + day.Format(time.DateOnly) + " were dropped"),
	})
}

// FoldAnalyticsDay folds a property's open local day up to FoldMargin before now.
func (s *Server) FoldAnalyticsDay(ctx context.Context, ws, propertyID uuid.UUID, day time.Time) error {
	if s.analytics == nil {
		return nil
	}
	prop, err := s.st.GetAnalyticsProperty(ctx, store.GetAnalyticsPropertyParams{WorkspaceID: ws, ID: propertyID})
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	pg := &analytics.PGStore{Pool: s.st.Pool}
	return pg.FoldDay(ctx, ws, propertyID, day, prop.Timezone, s.now().Add(-analytics.FoldMargin))
}

// DeleteExpiredAnalyticsSalts deletes every salt whose day ended more than the grace ago; without
// it a day's visitors can never be recomputed. Visit claims past the claim window go with them.
func (s *Server) DeleteExpiredAnalyticsSalts(ctx context.Context) (int64, error) {
	if s.analytics == nil {
		return 0, nil
	}
	ids, err := s.st.ListAnalyticsWorkspaceIDs(ctx)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, ws := range ids {
		n, err := s.st.DeleteExpiredAnalyticsSalts(ctx, store.DeleteExpiredAnalyticsSaltsParams{WorkspaceID: ws, Now: s.now()})
		if err != nil {
			return total, err
		}
		total += n
		if _, err := s.st.DeleteStaleAnalyticsVisitClaims(ctx, store.DeleteStaleAnalyticsVisitClaimsParams{
			WorkspaceID: ws, Before: s.now().Add(-analytics.ClaimWindow)}); err != nil {
			return total, err
		}
	}
	return total, nil
}

// UpdateGeoIP fetches the month's GeoIP database when this process does not have it yet, once at
// a time.
func (s *Server) UpdateGeoIP(ctx context.Context) (string, error) {
	if s.analytics == nil || s.analytics.geo == nil {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(ctx, geoipUpdateTimeout)
	defer cancel()
	path, err := s.analytics.geo.Update(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "analytics geoip", slog.Any("error", err))
	}
	return path, err
}
