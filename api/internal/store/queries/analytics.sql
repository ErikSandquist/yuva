-- name: CreateAnalyticsProperty :one
INSERT INTO analytics_properties (id, workspace_id, inbox_id, name, public_key, domains, platforms, timezone, currency, collection,
    consent_features, audience, excluded_paths, excluded_ips, path_rules, tracked_props, retention_months, visitor_key, created_by,
    created_at, updated_at)
VALUES (@id, @workspace_id, sqlc.narg(inbox_id), @name, @public_key, @domains, @platforms, @timezone, @currency, @collection,
    @consent_features, @audience, @excluded_paths, @excluded_ips, @path_rules, @tracked_props, sqlc.narg(retention_months), @visitor_key,
    sqlc.narg(created_by), @now, @now)
RETURNING *;

-- name: GetAnalyticsProperty :one
SELECT * FROM analytics_properties WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: LockAnalyticsProperty :one
SELECT * FROM analytics_properties WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: ListAnalyticsProperties :many
SELECT * FROM analytics_properties p
WHERE p.workspace_id = @workspace_id AND p.deleted_at IS NULL
  AND (@all_inboxes::bool OR EXISTS (
      SELECT 1 FROM inbox_viewers iv
      WHERE iv.workspace_id = p.workspace_id AND iv.inbox_id = p.inbox_id AND iv.viewer_id = @viewer_id))
ORDER BY p.created_at, p.id;

-- name: CountAnalyticsProperties :one
SELECT count(*) FROM analytics_properties WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

-- name: UpdateAnalyticsProperty :one
UPDATE analytics_properties SET
    inbox_id = sqlc.narg(inbox_id), name = @name, domains = @domains, platforms = @platforms, timezone = @timezone,
    currency = @currency, collection = @collection, consent_features = @consent_features, audience = @audience,
    excluded_paths = @excluded_paths, excluded_ips = @excluded_ips, path_rules = @path_rules, tracked_props = @tracked_props,
    retention_months = sqlc.narg(retention_months), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: RotateAnalyticsKey :one
UPDATE analytics_properties SET
    previous_key = public_key, previous_key_until = @previous_until, public_key = @public_key, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: MarkAnalyticsPropertyDeleted :execrows
UPDATE analytics_properties SET deleted_at = @now, previous_key = NULL, previous_key_until = NULL
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: FindAnalyticsPropertyByKey :one
-- Collection names only the public key, unique across the server; everything after this lookup is
-- scoped by the workspace it returns.
SELECT p.* FROM analytics_properties p JOIN workspaces w ON w.id = p.workspace_id
WHERE (p.public_key = @key OR (p.previous_key = @key AND p.previous_key_until > @now))
  AND p.deleted_at IS NULL AND w.deleted_at IS NULL;

-- name: GetDeletedAnalyticsProperty :one
SELECT * FROM analytics_properties WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NOT NULL;

-- name: DeleteAnalyticsPropertyRow :execrows
DELETE FROM analytics_properties WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NOT NULL;

-- name: ListWorkspaceAnalyticsPropertyIDs :many
SELECT id FROM analytics_properties WHERE workspace_id = @workspace_id ORDER BY id;

-- name: ListAnalyticsPropertyDays :many
SELECT day FROM analytics_rollup_days
WHERE workspace_id = @workspace_id AND property_id = @property_id
  AND (sqlc.narg(from_day)::date IS NULL OR day >= sqlc.narg(from_day)::date)
  AND (sqlc.narg(to_day)::date IS NULL OR day <= sqlc.narg(to_day)::date)
ORDER BY day;

-- name: MarkAnalyticsDaysDirty :execrows
UPDATE analytics_rollup_days SET dirty = true
WHERE workspace_id = @workspace_id AND property_id = @property_id AND day BETWEEN @from_day AND @to_day AND built_at IS NOT NULL;

-- name: DeleteAnalyticsRollups :exec
WITH d AS (DELETE FROM analytics_daily WHERE analytics_daily.workspace_id = @workspace_id AND analytics_daily.property_id = @property_id),
     h AS (DELETE FROM analytics_hourly WHERE analytics_hourly.workspace_id = @workspace_id AND analytics_hourly.property_id = @property_id)
DELETE FROM analytics_rollup_days WHERE analytics_rollup_days.workspace_id = @workspace_id AND analytics_rollup_days.property_id = @property_id;

-- name: GetAnalyticsUsage :one
SELECT w.analytics_monthly_events AS monthly_limit,
       coalesce(u.analytics_events, 0)::bigint AS events,
       u.analytics_limit_reached_at AS limit_reached_at
FROM workspaces w LEFT JOIN usage_counters u ON u.workspace_id = w.id AND u.month = @month
WHERE w.id = @workspace_id;

-- name: MarkAnalyticsLimitReached :execrows
UPDATE usage_counters SET analytics_limit_reached_at = @now
WHERE workspace_id = @workspace_id AND month = @month AND analytics_limit_reached_at IS NULL;

-- name: SetWorkspaceAnalyticsLimit :execrows
UPDATE workspaces SET analytics_monthly_events = sqlc.narg(monthly_events) WHERE id = @id AND deleted_at IS NULL;

-- name: ListOwnerAddresses :many
SELECT p.email, p.locale FROM members m JOIN people p ON p.id = m.person_id
WHERE m.workspace_id = @workspace_id AND m.role = 'owner'
ORDER BY m.created_at, m.id;

-- name: ListAnalyticsWorkspaceIDs :many
-- Maintenance finds the workspaces that have properties; everything after it is scoped by workspace.
SELECT DISTINCT workspace_id FROM analytics_properties ORDER BY workspace_id;

-- name: ListAnalyticsRollupTodo :many
SELECT d.property_id, d.day, p.timezone FROM analytics_rollup_days d
JOIN analytics_properties p ON p.workspace_id = d.workspace_id AND p.id = d.property_id
WHERE d.workspace_id = @workspace_id AND (d.built_at IS NULL OR d.dirty) AND p.deleted_at IS NULL AND d.day >= @oldest
ORDER BY d.property_id, d.day;

-- name: DeleteExpiredAnalyticsSalts :execrows
DELETE FROM analytics_salts WHERE workspace_id = @workspace_id AND expires_at <= @now;

-- name: ListAnalyticsRetention :many
SELECT id, retention_months, timezone FROM analytics_properties
WHERE workspace_id = @workspace_id AND retention_months IS NOT NULL AND deleted_at IS NULL
ORDER BY id;

-- name: TrimAnalyticsDaily :execrows
DELETE FROM analytics_daily WHERE ctid = ANY(ARRAY(
    SELECT t.ctid FROM analytics_daily t
    WHERE t.workspace_id = @workspace_id AND t.property_id = @property_id AND t.day < @before LIMIT 1000));

-- name: TrimAnalyticsHourly :execrows
DELETE FROM analytics_hourly WHERE ctid = ANY(ARRAY(
    SELECT t.ctid FROM analytics_hourly t
    WHERE t.workspace_id = @workspace_id AND t.property_id = @property_id AND t.day < @before LIMIT 1000));

-- name: TrimAnalyticsRollupDays :execrows
DELETE FROM analytics_rollup_days WHERE ctid = ANY(ARRAY(
    SELECT t.ctid FROM analytics_rollup_days t
    WHERE t.workspace_id = @workspace_id AND t.property_id = @property_id AND t.day < @before LIMIT 1000));

-- name: DeleteOldAnalyticsIngestStats :execrows
DELETE FROM analytics_ingest_stats WHERE workspace_id = @workspace_id AND day < @before;

-- name: ListAnalyticsIngestStats :many
SELECT day, reason, events FROM analytics_ingest_stats
WHERE workspace_id = @workspace_id AND property_id = @property_id AND day >= @from_day
ORDER BY day, reason;

-- name: CountPendingAnalyticsRollups :one
SELECT count(*) FROM analytics_rollup_days
WHERE workspace_id = @workspace_id AND property_id = @property_id AND (built_at IS NULL OR dirty);
