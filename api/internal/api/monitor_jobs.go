package api

import (
	"context"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/productdevbook/yuva/api/internal/jobs"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	monitorDeleteBatch = 50_000
	// maxMonitorHourDays is the longest monitor_hour_days; hour partitions older than that and a
	// month are dropped.
	maxMonitorHourDays = 400
)

// MonitorRollupArgs folds an API property's closed hours and finished days.
type MonitorRollupArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	PropertyID  uuid.UUID `json:"property_id"`
}

func (MonitorRollupArgs) Kind() string { return "monitor_rollup" }

func (MonitorRollupArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.AnalyticsQueue, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: analyticsRollupStates}}
}

type monitorRollupWorker struct {
	river.WorkerDefaults[MonitorRollupArgs]
	s *Server
}

func (w *monitorRollupWorker) Timeout(*river.Job[MonitorRollupArgs]) time.Duration {
	return analyticsRollupTimeout
}

func (w *monitorRollupWorker) Work(ctx context.Context, job *river.Job[MonitorRollupArgs]) error {
	return w.s.RollupMonitor(ctx, job.Args.WorkspaceID, job.Args.PropertyID)
}

// RollupMonitor folds a property's hours that closed and its local days that ended.
func (s *Server) RollupMonitor(ctx context.Context, ws, propertyID uuid.UUID) error {
	m, err := s.monitorOn()
	if err != nil {
		return nil
	}
	prop, err := s.st.GetAnalyticsProperty(ctx, store.GetAnalyticsPropertyParams{WorkspaceID: ws, ID: propertyID})
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	hours, days, err := m.store.Rollup(ctx, ws, propertyID, propertyLocation(prop.Timezone), s.now())
	if err != nil {
		return err
	}
	if hours+days > 0 {
		s.analytics.answers.forget(propertyID)
	}
	_ = river.RecordOutput(ctx, map[string]int{"hours": hours, "days": days})
	return nil
}

// scanMonitorRollups queues a fold for every API property of the workspace with an hour that closed
// and is not folded, or a finished day that is not.
func (s *Server) scanMonitorRollups(ctx context.Context, ws uuid.UUID) (int, error) {
	if s.analytics.monitor == nil {
		return 0, nil
	}
	ids, err := s.st.ListMonitorRollupProperties(ctx, store.ListMonitorRollupPropertiesParams{WorkspaceID: ws, Before: s.now().Add(-time.Hour - monitor.FoldAfter)})
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	batch := make([]river.InsertManyParams, len(ids))
	for i, id := range ids {
		batch[i] = river.InsertManyParams{Args: MonitorRollupArgs{WorkspaceID: ws, PropertyID: id}}
	}
	if _, err := s.jobs.InsertMany(ctx, batch); err != nil {
		return 0, err
	}
	return len(batch), nil
}

// maintainMonitor keeps minute partitions from yesterday to a week ahead and hour partitions for
// this month and the next, drops minute partitions past YUVA_MONITOR_MINUTE_DAYS and hour partitions
// past the longest hour retention.
func (s *Server) maintainMonitor(ctx context.Context, today time.Time) error {
	m := s.analytics.monitor
	if m == nil {
		return nil
	}
	if err := m.store.EnsureMinutePartitions(ctx, today.AddDate(0, 0, -1), today.AddDate(0, 0, partitionsAhead)); err != nil {
		return err
	}
	if err := m.store.EnsureHourPartitions(ctx, today, today.AddDate(0, 1, 0)); err != nil {
		return err
	}
	month := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	dropped, err := m.store.DropPartitionsBefore(ctx, today.AddDate(0, 0, 1-m.settings.MinuteDays), month.AddDate(0, 0, -maxMonitorHourDays).AddDate(0, -1, 0))
	if err != nil {
		return err
	}
	if len(dropped) > 0 {
		s.log.InfoContext(ctx, "monitor partitions dropped", slog.Int("count", len(dropped)))
	}
	return nil
}

// trimMonitor deletes an API property's hours past its monitor_hour_days and its days past its
// retention_months, and ingest counts past 90 days.
func (s *Server) trimMonitor(ctx context.Context, ws uuid.UUID, today time.Time) error {
	if s.analytics.monitor == nil {
		return nil
	}
	props, err := s.st.ListMonitorRetention(ctx, ws)
	if err != nil {
		return err
	}
	drain := func(trim func() (int64, error)) error {
		for {
			n, err := trim()
			if err != nil || n < 1000 {
				return err
			}
		}
	}
	for _, p := range props {
		loc := propertyLocation(p.Timezone)
		hourBefore := monitor.DayStart(monitor.LocalDay(s.now(), loc).AddDate(0, 0, -int(p.MonitorHourDays)), loc)
		if err := drain(func() (int64, error) {
			return s.st.TrimMonitorHour(ctx, store.TrimMonitorHourParams{WorkspaceID: ws, PropertyID: p.ID, Before: hourBefore})
		}); err != nil {
			return err
		}
		if err := drain(func() (int64, error) {
			return s.st.TrimMonitorDimHour(ctx, store.TrimMonitorDimHourParams{WorkspaceID: ws, PropertyID: p.ID, Before: hourBefore})
		}); err != nil {
			return err
		}
		if p.RetentionMonths == nil {
			continue
		}
		dayBefore := monitor.LocalDay(s.now(), loc).AddDate(0, -int(*p.RetentionMonths), 0)
		if err := drain(func() (int64, error) {
			return s.st.TrimMonitorDay(ctx, store.TrimMonitorDayParams{WorkspaceID: ws, PropertyID: p.ID, Before: dayBefore})
		}); err != nil {
			return err
		}
		if err := drain(func() (int64, error) {
			return s.st.TrimMonitorDimDay(ctx, store.TrimMonitorDimDayParams{WorkspaceID: ws, PropertyID: p.ID, Before: dayBefore})
		}); err != nil {
			return err
		}
		if err := drain(func() (int64, error) {
			return s.st.TrimMonitorRollupHours(ctx, store.TrimMonitorRollupHoursParams{WorkspaceID: ws, PropertyID: p.ID, Before: monitor.DayStart(dayBefore, loc)})
		}); err != nil {
			return err
		}
	}
	_, err = s.st.DeleteOldMonitorIngestStats(ctx, store.DeleteOldMonitorIngestStatsParams{WorkspaceID: ws, Before: today.AddDate(0, 0, -analyticsStatsDays)})
	return err
}

// purgeMonitor deletes a property's request rows: of the local days from to to, or all of them.
func (s *Server) purgeMonitor(ctx context.Context, ws, propertyID uuid.UUID, from, to *time.Time, deadline time.Time) (bool, error) {
	var fromAt, toAt *time.Time
	if from != nil {
		prop, err := s.st.Queries.GetAnalyticsProperty(ctx, store.GetAnalyticsPropertyParams{WorkspaceID: ws, ID: propertyID})
		if store.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		loc := propertyLocation(prop.Timezone)
		fromAt, toAt = new(monitor.DayStart(*from, loc)), new(monitor.DayStart(to.AddDate(0, 0, 1), loc))
	}
	for {
		n, err := s.st.DeleteMonitorMinutes(ctx, store.DeleteMonitorMinutesParams{WorkspaceID: ws, PropertyID: propertyID, FromAt: fromAt, ToAt: toAt, MaxRows: monitorDeleteBatch})
		if err != nil {
			return false, err
		}
		if n < monitorDeleteBatch {
			break
		}
		if time.Now().After(deadline) {
			return false, nil
		}
	}
	if from == nil {
		return true, s.st.DeleteMonitorRollups(ctx, store.DeleteMonitorRollupsParams{WorkspaceID: ws, PropertyID: propertyID})
	}
	return true, s.st.DeleteMonitorRange(ctx, store.DeleteMonitorRangeParams{WorkspaceID: ws, PropertyID: propertyID, FromAt: *fromAt, ToAt: *toAt,
		FromDay: *from, ToDay: *to})
}
