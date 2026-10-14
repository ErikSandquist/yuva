-- name: GetMonitorUsage :one
SELECT w.monitor_monthly_requests AS monthly_limit,
       coalesce(u.monitor_requests, 0)::bigint AS requests,
       u.monitor_limit_reached_at AS limit_reached_at
FROM workspaces w LEFT JOIN usage_counters u ON u.workspace_id = w.id AND u.month = @month
WHERE w.id = @workspace_id;

-- name: MarkMonitorLimitReached :execrows
UPDATE usage_counters SET monitor_limit_reached_at = @now
WHERE workspace_id = @workspace_id AND month = @month AND monitor_limit_reached_at IS NULL;

-- name: ListMonitorRoutes :many
SELECT id, pattern, inferred, hidden, apdex_ms, first_seen_at, last_seen_at FROM monitor_routes
WHERE workspace_id = @workspace_id AND property_id = @property_id
ORDER BY pattern;

-- name: UpdateMonitorRoute :one
UPDATE monitor_routes SET hidden = coalesce(sqlc.narg(hidden)::boolean, hidden),
    apdex_ms = CASE WHEN @set_apdex::boolean THEN sqlc.narg(apdex_ms)::int ELSE apdex_ms END
WHERE workspace_id = @workspace_id AND property_id = @property_id AND id = @id
RETURNING id, pattern, inferred, hidden, apdex_ms, first_seen_at, last_seen_at;

-- name: SetWorkspaceMonitorLimit :execrows
UPDATE workspaces SET monitor_monthly_requests = sqlc.narg(monthly_requests) WHERE id = @id AND deleted_at IS NULL;

-- name: ListMonitorRollupProperties :many
SELECT DISTINCT h.property_id FROM monitor_rollup_hours h
JOIN analytics_properties p ON p.workspace_id = h.workspace_id AND p.id = h.property_id
WHERE h.workspace_id = @workspace_id AND p.deleted_at IS NULL AND h.hour < @before
  AND (h.built_at IS NULL OR h.dirty OR h.day_built_at IS NULL)
ORDER BY h.property_id;

-- name: ListMonitorRetention :many
SELECT id, retention_months, monitor_hour_days, timezone, timezone_history FROM analytics_properties
WHERE workspace_id = @workspace_id AND kind = 'api' AND deleted_at IS NULL
ORDER BY id;

-- name: TrimMonitorDay :execrows
DELETE FROM monitor_day WHERE ctid = ANY(ARRAY(
    SELECT t.ctid FROM monitor_day t
    WHERE t.workspace_id = @workspace_id AND t.property_id = @property_id AND t.day < @before LIMIT 1000));

-- name: TrimMonitorDimDay :execrows
DELETE FROM monitor_dim_day WHERE ctid = ANY(ARRAY(
    SELECT t.ctid FROM monitor_dim_day t
    WHERE t.workspace_id = @workspace_id AND t.property_id = @property_id AND t.day < @before LIMIT 1000));

-- name: TrimMonitorHour :execrows
DELETE FROM monitor_hour WHERE (tableoid, ctid) IN (
    SELECT t.tableoid, t.ctid FROM monitor_hour t
    WHERE t.workspace_id = @workspace_id AND t.property_id = @property_id AND t.hour < @before LIMIT 1000);

-- name: TrimMonitorDimHour :execrows
DELETE FROM monitor_dim_hour WHERE (tableoid, ctid) IN (
    SELECT t.tableoid, t.ctid FROM monitor_dim_hour t
    WHERE t.workspace_id = @workspace_id AND t.property_id = @property_id AND t.hour < @before LIMIT 1000);

-- name: TrimMonitorRollupHours :execrows
DELETE FROM monitor_rollup_hours WHERE ctid = ANY(ARRAY(
    SELECT t.ctid FROM monitor_rollup_hours t
    WHERE t.workspace_id = @workspace_id AND t.property_id = @property_id AND t.hour < @before LIMIT 1000));

-- name: DeleteOldMonitorIngestStats :execrows
DELETE FROM monitor_ingest_stats WHERE workspace_id = @workspace_id AND day < @before;

-- name: DeleteMonitorRollups :exec
WITH d AS (DELETE FROM monitor_day WHERE monitor_day.workspace_id = @workspace_id AND monitor_day.property_id = @property_id),
     dd AS (DELETE FROM monitor_dim_day WHERE monitor_dim_day.workspace_id = @workspace_id AND monitor_dim_day.property_id = @property_id),
     h AS (DELETE FROM monitor_hour WHERE monitor_hour.workspace_id = @workspace_id AND monitor_hour.property_id = @property_id),
     dh AS (DELETE FROM monitor_dim_hour WHERE monitor_dim_hour.workspace_id = @workspace_id AND monitor_dim_hour.property_id = @property_id)
DELETE FROM monitor_rollup_hours WHERE monitor_rollup_hours.workspace_id = @workspace_id AND monitor_rollup_hours.property_id = @property_id;

-- name: DeleteMonitorMinutes :execrows
DELETE FROM monitor_minute WHERE (tableoid, ctid) IN (
    SELECT t.tableoid, t.ctid FROM monitor_minute t
    WHERE t.workspace_id = @workspace_id AND t.property_id = @property_id
      AND (sqlc.narg(from_at)::timestamptz IS NULL OR t.minute >= sqlc.narg(from_at)::timestamptz)
      AND (sqlc.narg(to_at)::timestamptz IS NULL OR t.minute < sqlc.narg(to_at)::timestamptz)
    LIMIT @max_rows::int);

-- name: DeleteMonitorRange :exec
WITH h AS (DELETE FROM monitor_hour WHERE monitor_hour.workspace_id = @workspace_id AND monitor_hour.property_id = @property_id
               AND monitor_hour.hour >= @from_at AND monitor_hour.hour < @to_at),
     dh AS (DELETE FROM monitor_dim_hour WHERE monitor_dim_hour.workspace_id = @workspace_id AND monitor_dim_hour.property_id = @property_id
               AND monitor_dim_hour.hour >= @from_at AND monitor_dim_hour.hour < @to_at),
     d AS (DELETE FROM monitor_day WHERE monitor_day.workspace_id = @workspace_id AND monitor_day.property_id = @property_id
               AND monitor_day.day BETWEEN @from_day AND @to_day),
     dd AS (DELETE FROM monitor_dim_day WHERE monitor_dim_day.workspace_id = @workspace_id AND monitor_dim_day.property_id = @property_id
               AND monitor_dim_day.day BETWEEN @from_day AND @to_day)
DELETE FROM monitor_rollup_hours WHERE monitor_rollup_hours.workspace_id = @workspace_id AND monitor_rollup_hours.property_id = @property_id
    AND monitor_rollup_hours.day BETWEEN @from_day AND @to_day;

-- name: BumpAnalyticsRollupVersion :one
UPDATE analytics_properties SET rollup_version = rollup_version + 1
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING rollup_version;

-- name: ListMonitorConsumers :many
SELECT id, value::text AS name, first_seen_at FROM monitor_names
WHERE workspace_id = @workspace_id AND property_id = @property_id AND kind = 'consumer' AND value IS NOT NULL AND value <> '(other)'
  AND (sqlc.narg(search)::text IS NULL OR strpos(lower(value), lower(sqlc.narg(search)::text)) > 0)
  AND (sqlc.narg(after)::text IS NULL OR value > sqlc.narg(after)::text COLLATE "C")
ORDER BY value
LIMIT @max_rows;

-- name: GetMonitorConsumerForUpdate :one
SELECT id, value::text AS name FROM monitor_names
WHERE workspace_id = @workspace_id AND property_id = @property_id AND kind = 'consumer' AND id = @id AND value IS NOT NULL
FOR UPDATE;

-- name: EraseMonitorConsumerName :exec
UPDATE monitor_names SET value = NULL, erased_at = @now
WHERE workspace_id = @workspace_id AND property_id = @property_id AND kind = 'consumer' AND id = @id;

-- name: DeleteMonitorConsumerRows :exec
WITH h AS (DELETE FROM monitor_dim_hour WHERE monitor_dim_hour.workspace_id = @workspace_id AND monitor_dim_hour.property_id = @property_id
               AND monitor_dim_hour.dim = 1 AND monitor_dim_hour.value = @consumer_id::int)
DELETE FROM monitor_dim_day WHERE monitor_dim_day.workspace_id = @workspace_id AND monitor_dim_day.property_id = @property_id
    AND monitor_dim_day.dim = 1 AND monitor_dim_day.value = @consumer_id::int;
