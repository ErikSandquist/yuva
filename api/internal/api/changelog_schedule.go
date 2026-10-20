package api

import (
	"context"
	"slices"
	"time"
	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/productdevbook/yuva/api/internal/jobs"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const changelogSweepBatch = 500

type ChangelogPublishArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	EntryID     uuid.UUID `json:"entry_id"`
	// Sweep marks the job the sweep queues, which is unique while it waits; the job a schedule queues is not.
	Sweep bool `json:"sweep,omitempty"`
}

func (ChangelogPublishArgs) Kind() string { return "changelog_publish" }

// A schedule that moves leaves its first job behind; it finds the entry not yet due and does nothing.
func changelogJobOpts(at time.Time) *river.InsertOpts {
	return &river.InsertOpts{ScheduledAt: at, MaxAttempts: 5}
}

type changelogPublishWorker struct {
	river.WorkerDefaults[ChangelogPublishArgs]
	s *Server
}

func (w *changelogPublishWorker) Work(ctx context.Context, job *river.Job[ChangelogPublishArgs]) error {
	return w.s.PublishScheduledEntry(ctx, job.Args.WorkspaceID, job.Args.EntryID)
}

type changelogSweepWorker struct {
	river.WorkerDefaults[jobs.ChangelogSweepArgs]
	s *Server
}

func (w *changelogSweepWorker) Work(ctx context.Context, _ *river.Job[jobs.ChangelogSweepArgs]) error {
	return w.s.SweepChangelog(ctx)
}

// It reads across workspaces; each job then works in the entry's own workspace.
func (s *Server) SweepChangelog(ctx context.Context) error {
	rows, err := s.st.DueChangelogEntries(ctx, store.DueChangelogEntriesParams{Now: s.now(), Lim: changelogSweepBatch})
	if err != nil || len(rows) == 0 {
		return err
	}
	opts := &river.InsertOpts{MaxAttempts: 5, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}}}
	batch := make([]river.InsertManyParams, len(rows))
	for i, r := range rows {
		batch[i] = river.InsertManyParams{Args: ChangelogPublishArgs{WorkspaceID: r.WorkspaceID, EntryID: r.ID, Sweep: true}, InsertOpts: opts}
	}
	_, err = s.jobs.InsertMany(ctx, batch)
	return err
}

// schedulerMayPublish asks again whether the one who scheduled the entry may publish it now.
// It answers the reason they may not, or "".
func (s *Server) schedulerMayPublish(ctx context.Context, q *store.Queries, c store.Changelog, e store.ChangelogEntry) (string, error) {
	switch {
	case e.ScheduledByApiKeyID != nil:
		k, err := q.GetAPIKey(ctx, store.GetAPIKeyParams{WorkspaceID: c.WorkspaceID, ID: *e.ScheduledByApiKeyID})
		if store.IsNotFound(err) {
			return string(oas.ChangelogFailureKeyRevoked), nil
		}
		if err != nil {
			return "", err
		}
		if k.RevokedAt != nil || (k.ExpiresAt != nil && !s.now().Before(*k.ExpiresAt)) {
			return string(oas.ChangelogFailureKeyRevoked), nil
		}
		p := principal{keyID: k.ID, workspaceID: c.WorkspaceID, scopes: k.Scopes, limited: k.InboxLimited}
		if !slices.Contains(k.Scopes, string(oas.ChangelogPublish)) {
			return string(oas.ChangelogFailureKeyScopeLost), nil
		}
		ok, err := canSeeInbox(ctx, q, p, c.InboxID)
		if err != nil || !ok {
			return string(oas.ChangelogFailureKeyScopeLost), err
		}
	case e.ScheduledByMemberID != nil:
		m, err := q.GetMember(ctx, store.GetMemberParams{WorkspaceID: c.WorkspaceID, ID: *e.ScheduledByMemberID})
		if store.IsNotFound(err) {
			return string(oas.ChangelogFailureLostRight), nil
		}
		if err != nil {
			return "", err
		}
		p := principal{workspaceID: c.WorkspaceID, memberID: m.ID, role: m.Role}
		if mayPublishChangelog(p, c) != nil {
			return string(oas.ChangelogFailureLostRight), nil
		}
		ok, err := canSeeInbox(ctx, q, p, c.InboxID)
		if err != nil || !ok {
			return string(oas.ChangelogFailureLostRight), err
		}
	default:
		return string(oas.ChangelogFailureLostRight), nil
	}
	return "", nil
}

// PublishScheduledEntry publishes a scheduled entry that is due, once: the entry is locked and
// moved by a conditional update, so a second job, the sweep or a retry finds nothing to do. An
// entry whose scheduler may no longer publish goes back to draft with the reason.
func (s *Server) PublishScheduledEntry(ctx context.Context, ws, entryID uuid.UUID) error {
	var touched *store.Changelog
	var published bool
	err := s.inTx(ctx, ws, func(q *store.Queries, ev *eventBatch) error {
		found, err := q.GetChangelogEntryByID(ctx, store.GetChangelogEntryByIDParams{WorkspaceID: ws, ID: entryID})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		c, err := q.LockChangelog(ctx, store.LockChangelogParams{WorkspaceID: ws, ID: found.ChangelogID})
		if err != nil {
			return err
		}
		e, err := q.LockChangelogEntry(ctx, store.LockChangelogEntryParams{WorkspaceID: ws, ChangelogID: c.ID, ID: entryID})
		if err != nil {
			return err
		}
		now := s.now()
		if e.State != changelogStateScheduled || e.PublishAt == nil || e.PublishAt.After(now) {
			return nil
		}
		reason, err := s.schedulerMayPublish(ctx, q, c, e)
		if err != nil {
			return err
		}
		if reason == "" {
			ok, err := s.hasDefaultChangelogTranslation(ctx, q, c, e)
			if err != nil {
				return err
			}
			if !ok {
				reason = string(oas.ChangelogFailureNoDefaultTranslation)
			}
		}
		touched = &c
		if reason != "" {
			_, err := q.FailChangelogSchedule(ctx, store.FailChangelogScheduleParams{WorkspaceID: ws, ID: e.ID, Reason: &reason, Now: now})
			return err
		}
		due, err := q.PublishDueChangelogEntry(ctx, store.PublishDueChangelogEntryParams{WorkspaceID: ws, ID: e.ID, Now: now})
		if store.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.freezeChangelogEntry(ctx, q, ev, c, due, changelogActor{member: due.ScheduledByMemberID, key: due.ScheduledByApiKeyID}, ""); err != nil {
			return err
		}
		published = true
		return nil
	})
	if err != nil {
		return err
	}
	if touched != nil {
		s.changelogSignal(ctx, principal{workspaceID: ws}, *touched, entryID, nil)
	}
	if published {
		s.statusHint(ctx, ws, touched.InboxID)
	}
	return nil
}
