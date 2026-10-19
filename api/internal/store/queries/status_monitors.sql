-- name: CreateStatusMonitor :one
INSERT INTO status_monitors (id, workspace_id, page_id, component_id, kind, name, token_hash, token_prefix, interval_seconds, grace_seconds,
    failure_state, failure_threshold, recovery_threshold, on_failure, paused, state, due_at, changed_at, created_at, updated_at)
VALUES (@id, @workspace_id, @page_id, @component_id, @kind, @name, @token_hash, @token_prefix, sqlc.narg(interval_seconds), sqlc.narg(grace_seconds),
    @failure_state, @failure_threshold, @recovery_threshold, @on_failure, @paused, 'pending', sqlc.narg(due_at), @now, @now, @now)
RETURNING *;

-- name: CountStatusMonitors :one
SELECT count(*)::bigint FROM status_monitors WHERE workspace_id = @workspace_id;

-- name: GetStatusMonitor :one
SELECT * FROM status_monitors WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id;

-- name: GetStatusMonitorByID :one
SELECT * FROM status_monitors WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockStatusMonitor :one
SELECT * FROM status_monitors WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: ListStatusMonitors :many
SELECT * FROM status_monitors WHERE workspace_id = @workspace_id AND page_id = @page_id ORDER BY created_at, id;

-- name: FindStatusMonitorByToken :one
-- The documented exception: a heartbeat or push finds its monitor by its token before the workspace is known.
SELECT workspace_id, id, kind FROM status_monitors WHERE token_hash = @token_hash;

-- name: UpdateStatusMonitorSettings :one
UPDATE status_monitors SET name = @name, component_id = @component_id, failure_state = @failure_state, failure_threshold = @failure_threshold,
    recovery_threshold = @recovery_threshold, on_failure = @on_failure, interval_seconds = sqlc.narg(interval_seconds),
    grace_seconds = sqlc.narg(grace_seconds), due_at = sqlc.narg(due_at), due_extended = @due_extended, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetStatusMonitorPaused :one
UPDATE status_monitors SET paused = @paused, state = @state, down_state = sqlc.narg(down_state), consecutive_failures = 0,
    consecutive_successes = 0, due_at = sqlc.narg(due_at), due_extended = false, incident_id = NULL, changed_at = @changed_at, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetStatusMonitorToken :one
UPDATE status_monitors SET token_hash = @token_hash, token_prefix = @token_prefix, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetStatusMonitorResult :one
UPDATE status_monitors SET state = @state, down_state = sqlc.narg(down_state), consecutive_failures = @consecutive_failures,
    consecutive_successes = @consecutive_successes, last_result_at = @at::timestamptz, last_message = sqlc.narg(last_message),
    due_at = sqlc.narg(due_at), due_extended = @due_extended, incident_id = sqlc.narg(incident_id), changed_at = @changed_at, updated_at = @at
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ExtendStatusMonitorDue :exec
UPDATE status_monitors SET due_at = @due_at::timestamptz, due_extended = true
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteStatusMonitor :execrows
DELETE FROM status_monitors WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id;

-- name: ListDueStatusMonitors :many
-- The documented exception: the sweep finds the heartbeats whose beat is overdue in every workspace at once.
SELECT workspace_id, id FROM status_monitors
WHERE due_at IS NOT NULL AND NOT paused AND due_at <= @now::timestamptz
ORDER BY due_at
LIMIT 500;

-- name: FailingStatusMonitors :many
-- A monitor is overridden, and holds nothing, once a member or key set its component's state after the monitor's last change.
SELECT m.id, m.component_id, m.down_state::text AS state, m.changed_at FROM status_monitors m
WHERE m.workspace_id = @workspace_id AND m.component_id = ANY(@component_ids::uuid[]) AND m.state = 'down' AND NOT m.paused
  AND NOT EXISTS (
    SELECT 1 FROM status_component_states s
    WHERE s.workspace_id = m.workspace_id AND s.component_id = m.component_id AND s.source IN ('member', 'api_key') AND s.started_at > m.changed_at)
ORDER BY m.changed_at, m.id;

-- name: CreateStatusCheck :exec
INSERT INTO status_checks (id, workspace_id, monitor_id, at, ok, source, state, message, error)
VALUES (@id, @workspace_id, @monitor_id, @at, @ok, @source, sqlc.narg(state), sqlc.narg(message), sqlc.narg(error));

-- name: ListStatusChecks :many
SELECT * FROM status_checks
WHERE workspace_id = @workspace_id AND monitor_id = @monitor_id
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL
       OR (at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY at DESC, id DESC
LIMIT @lim;

-- name: DeleteOldStatusChecks :execrows
DELETE FROM status_checks WHERE id IN (
    SELECT c.id FROM status_checks c WHERE c.workspace_id = @workspace_id AND c.at < @before LIMIT 10000);
