package api

import (
	"context"
	"log/slog"
	"math"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/productdevbook/yuva/api/internal/jobs"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	checkRetryDelay  = 20 * time.Second
	checksDueBatch   = 500
	checkJobSlack    = 15 * time.Second
	checkSettleDelay = 40 * time.Second
	// A minute in which at least this many checks ran and more than half failed is recorded
	// unknown: the server's own network is the likelier fault.
	unknownMinuteRuns = 20
	checkDaysKept     = 400
	monitorAlertsKept = 400 * 24 * time.Hour

	resultUp      = "up"
	resultDown    = "down"
	resultUnknown = "unknown"
	checkUnknown  = "unknown"
)

// MonitorCheckArgs runs one uptime check of one slot; attempt 2 is the try 20 seconds after a
// failed one.
type MonitorCheckArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	CheckID     uuid.UUID `json:"check_id"`
	Slot        int64     `json:"slot"`
	Attempt     int       `json:"attempt"`
}

func (MonitorCheckArgs) Kind() string { return "monitor_check" }

func (MonitorCheckArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.MonitoringQueue, MaxAttempts: 1, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type monitorCheckWorker struct {
	river.WorkerDefaults[MonitorCheckArgs]
	s *Server
}

func (w *monitorCheckWorker) Timeout(*river.Job[MonitorCheckArgs]) time.Duration {
	return 30*time.Second + checkJobSlack
}

func (w *monitorCheckWorker) Work(ctx context.Context, job *river.Job[MonitorCheckArgs]) error {
	return w.s.RunMonitorCheck(ctx, job.Args)
}

type monitorChecksDueWorker struct {
	river.WorkerDefaults[jobs.MonitorChecksDueArgs]
	s *Server
}

func (w *monitorChecksDueWorker) Work(ctx context.Context, _ *river.Job[jobs.MonitorChecksDueArgs]) error {
	if _, err := w.s.QueueDueMonitorChecks(ctx); err != nil {
		return err
	}
	return w.s.SettleMonitorResults(ctx)
}

type monitorAlertsWorker struct {
	river.WorkerDefaults[jobs.MonitorAlertsArgs]
	s *Server
}

func (w *monitorAlertsWorker) Work(ctx context.Context, _ *river.Job[jobs.MonitorAlertsArgs]) error {
	_, err := w.s.CheckMonitorAlerts(ctx)
	return err
}

func (s *Server) addMonitorWorkers(workers *river.Workers) {
	river.AddWorker(workers, &monitorCheckWorker{s: s})
	river.AddWorker(workers, &monitorChecksDueWorker{s: s})
	river.AddWorker(workers, &monitorAlertsWorker{s: s})
	river.AddWorker(workers, &monitorEraseWorker{s: s})
}

// QueueDueMonitorChecks takes the due checks of every workspace, moves each one's next run on by
// its interval (skipping slots that passed) and queues one run per check and slot.
func (s *Server) QueueDueMonitorChecks(ctx context.Context) (int, error) {
	if _, err := s.monitorOn(); err != nil {
		return 0, nil
	}
	now := s.now()
	queued := 0
	err := s.st.InTxRaw(ctx, func(tx pgx.Tx, q *store.Queries) error {
		due, err := q.ListDueMonitorChecks(ctx, store.ListDueMonitorChecksParams{Now: now, Lim: checksDueBatch})
		if err != nil || len(due) == 0 {
			return err
		}
		batch := make([]river.InsertManyParams, 0, len(due))
		for _, c := range due {
			every := time.Duration(c.IntervalSeconds) * time.Second
			next := c.NextRunAt.Add(every)
			if !next.After(now) {
				next = now.Add(every)
			}
			if err := q.SetMonitorCheckNextRun(ctx, store.SetMonitorCheckNextRunParams{WorkspaceID: c.WorkspaceID, ID: c.ID, NextRunAt: next}); err != nil {
				return err
			}
			batch = append(batch, river.InsertManyParams{Args: MonitorCheckArgs{WorkspaceID: c.WorkspaceID, CheckID: c.ID, Slot: c.NextRunAt.Unix(), Attempt: 1}})
		}
		queued = len(batch)
		_, err = s.jobs.InsertManyTx(ctx, tx, batch)
		return err
	})
	return queued, err
}

func (s *Server) checkSpec(c store.MonitorCheck) monitor.ProbeSpec {
	spec := monitor.ProbeSpec{URL: c.Url, Method: c.Method, KeywordAbsent: c.KeywordAbsent, Timeout: time.Duration(c.TimeoutSeconds) * time.Second,
		MaxRedirects: int(c.MaxRedirects), UserAgent: "Yuva-Uptime/" + s.version}
	for _, v := range c.ExpectedStatuses {
		spec.Expected = append(spec.Expected, int(v))
	}
	if c.Keyword != nil {
		spec.Keyword = *c.Keyword
	}
	for _, h := range headersOf(c.Headers) {
		v, err := s.secrets.Open(h.Value, checkSecretContext(c.WorkspaceID, c.ID))
		if err != nil {
			continue
		}
		spec.Headers = append(spec.Headers, [2]string{h.Name, string(v)})
	}
	return spec
}

func (s *Server) ensureCheckPartition(ctx context.Context, m *monitorState, at time.Time) error {
	day := utcDay(at)
	if _, ok := m.parts.Load(day); ok {
		return nil
	}
	if err := m.store.EnsureCheckPartitions(ctx, day, day.AddDate(0, 0, 1)); err != nil {
		return err
	}
	m.parts.Store(day, true)
	return nil
}

// RunMonitorCheck makes one try of a check. A failed first try queues a second one 20 seconds
// later and records nothing; every other try is recorded as the run of its slot, to be judged once
// its minute closed.
func (s *Server) RunMonitorCheck(ctx context.Context, a MonitorCheckArgs) error {
	m, err := s.monitorOn()
	if err != nil {
		return nil
	}
	c, err := s.st.GetMonitorCheck(ctx, store.GetMonitorCheckParams{WorkspaceID: a.WorkspaceID, ID: a.CheckID})
	if store.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if c.Paused {
		return nil
	}
	at := s.now()
	res := m.prober.Probe(ctx, s.checkSpec(c))
	if !res.OK && a.Attempt <= 1 {
		retry := a
		retry.Attempt = 2
		_, err := s.jobs.Insert(ctx, retry, &river.InsertOpts{ScheduledAt: at.Add(checkRetryDelay)})
		return err
	}
	if err := s.ensureCheckPartition(ctx, m, at); err != nil {
		return err
	}
	us := func(d time.Duration) int64 { return d.Microseconds() }
	arg := store.InsertMonitorCheckResultParams{
		WorkspaceID: c.WorkspaceID, CheckID: c.ID, At: at, Ok: res.OK, Status: int32(res.Status), Retried: a.Attempt > 1, Redirects: int32(res.Redirects),
		DnsUs: us(res.DNS), ConnectUs: us(res.Connect), TlsUs: us(res.TLS), TtfbUs: us(res.TTFB), TotalUs: us(res.Total), Answered: res.Answered,
		Seconds: c.IntervalSeconds,
	}
	if !res.OK {
		arg.Error, arg.Detail = &res.Error, new(truncateRunes(res.Detail, 200))
	}
	if res.CertExpires != nil {
		arg.CertDaysLeft = new(int32(math.Floor(res.CertExpires.Sub(at).Hours() / 24)))
	}
	err = s.st.InTx(ctx, func(q *store.Queries) error {
		if err := q.InsertMonitorCheckResult(ctx, arg); err != nil {
			return err
		}
		return q.MarkMonitorCheckRun(ctx, store.MarkMonitorCheckRunParams{WorkspaceID: c.WorkspaceID, ID: c.ID, At: &at, CertExpiresAt: res.CertExpires})
	})
	return err
}

// nextCheckState is a check's state and failed runs in a row after a counted run: down once
// failures runs in a row failed, up after any success, and unchanged before either.
func nextCheckState(state string, inRow, failures int32, ok bool) (string, int32) {
	if ok {
		return resultUp, 0
	}
	inRow++
	if inRow >= failures {
		return resultDown, inRow
	}
	return state, inRow
}

// SettleMonitorResults judges the runs of the minutes that closed, oldest first: a minute where
// at least 20 checks of the server ran and more than half failed records its runs unknown; the
// other runs move their check's state in order.
func (s *Server) SettleMonitorResults(ctx context.Context) error {
	if _, err := s.monitorOn(); err != nil {
		return nil
	}
	before := s.now().Add(-checkSettleDelay).Truncate(time.Minute)
	minutes, err := s.st.ListPendingMonitorMinutes(ctx, before)
	if err != nil {
		return err
	}
	for _, minute := range minutes {
		end := minute.Add(time.Minute)
		count, err := s.st.CountMonitorMinute(ctx, store.CountMonitorMinuteParams{FromAt: minute, ToAt: end})
		if err != nil {
			return err
		}
		unknown := count.Runs >= unknownMinuteRuns && count.Failures*2 > count.Runs
		pending, err := s.st.ListPendingMonitorResults(ctx, store.ListPendingMonitorResultsParams{FromAt: minute, ToAt: end})
		if err != nil {
			return err
		}
		for start := 0; start < len(pending); {
			stop := start
			for stop < len(pending) && pending[stop].CheckID == pending[start].CheckID {
				stop++
			}
			if err := s.settleCheck(ctx, pending[start:stop], unknown); err != nil {
				return err
			}
			start = stop
		}
	}
	return nil
}

func (s *Server) settleCheck(ctx context.Context, runs []store.ListPendingMonitorResultsRow, unknown bool) error {
	ws, id := runs[0].WorkspaceID, runs[0].CheckID
	var before, after store.MonitorCheck
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		c, err := q.LockMonitorCheck(ctx, store.LockMonitorCheckParams{WorkspaceID: ws, ID: id})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		before, after = c, c
		state, inRow, since := c.State, c.FailuresInRow, c.StateSince
		for _, r := range runs {
			recorded := resultUnknown
			if !unknown {
				next, n := nextCheckState(state, inRow, c.FailuresBeforeDown, r.Ok)
				if next != state {
					since = &r.At
				}
				state, inRow = next, n
				recorded = resultUp
				if state == resultDown {
					recorded = resultDown
				}
			}
			if err := q.SetMonitorResultState(ctx, store.SetMonitorResultStateParams{WorkspaceID: ws, CheckID: id, At: r.At, State: &recorded}); err != nil {
				return err
			}
		}
		after, err = q.SetMonitorCheckState(ctx, store.SetMonitorCheckStateParams{WorkspaceID: ws, ID: id, State: state, StateSince: since, FailuresInRow: inRow})
		return err
	})
	if err != nil || after.ID == uuid.Nil() || before.State == after.State {
		return err
	}
	if err := s.checkRulesOn(ctx, after); err != nil {
		s.log.WarnContext(ctx, "monitor check alerts", slog.String("workspace_id", ws.String()), slog.Any("error", err))
	}
	return nil
}

// foldMonitorChecks folds the UTC days that ended into each check's daily rows, once every run of
// the day was judged.
func (s *Server) foldMonitorChecks(ctx context.Context, ws uuid.UUID, today time.Time) error {
	m := s.analytics.monitor
	yesterday := today.AddDate(0, 0, -1)
	oldest := today.AddDate(0, 0, -m.settings.CheckDays)
	checks, err := s.st.ListMonitorChecksToFold(ctx, store.ListMonitorChecksToFoldParams{WorkspaceID: ws, LastDay: yesterday})
	if err != nil {
		return err
	}
	for _, c := range checks {
		day := utcDay(c.CreatedAt)
		if c.FoldedUntil != nil {
			day = c.FoldedUntil.AddDate(0, 0, 1)
		}
		if day.Before(oldest) {
			day = oldest
		}
		for ; !day.After(yesterday); day = day.AddDate(0, 0, 1) {
			n, err := s.st.CountPendingMonitorDay(ctx, store.CountPendingMonitorDayParams{WorkspaceID: ws, CheckID: c.ID, FromAt: day, ToAt: day.AddDate(0, 0, 1)})
			if err != nil {
				return err
			}
			if n > 0 {
				break
			}
			if err := m.store.FoldCheckDay(ctx, ws, c.ID, day); err != nil {
				return err
			}
		}
	}
	return nil
}

// maintainMonitorChecks keeps run partitions from yesterday to a week ahead and drops those past
// YUVA_MONITOR_CHECK_DAYS.
func (s *Server) maintainMonitorChecks(ctx context.Context, today time.Time) error {
	m := s.analytics.monitor
	if err := m.store.EnsureCheckPartitions(ctx, today.AddDate(0, 0, -1), today.AddDate(0, 0, partitionsAhead)); err != nil {
		return err
	}
	dropped, err := m.store.DropCheckPartitionsBefore(ctx, today.AddDate(0, 0, 1-m.settings.CheckDays))
	if err != nil {
		return err
	}
	if len(dropped) > 0 {
		s.log.InfoContext(ctx, "monitor check partitions dropped", slog.Int("count", len(dropped)))
	}
	return nil
}

// trimMonitorChecks folds finished days and deletes check days and resolved alerts past 400 days.
func (s *Server) trimMonitorChecks(ctx context.Context, ws uuid.UUID, today time.Time) error {
	if err := s.foldMonitorChecks(ctx, ws, today); err != nil {
		return err
	}
	if _, err := s.st.DeleteOldMonitorCheckDays(ctx, store.DeleteOldMonitorCheckDaysParams{WorkspaceID: ws, Before: today.AddDate(0, 0, -checkDaysKept)}); err != nil {
		return err
	}
	_, err := s.st.DeleteOldMonitorAlerts(ctx, store.DeleteOldMonitorAlertsParams{WorkspaceID: ws, Before: new(s.now().Add(-monitorAlertsKept))})
	return err
}
