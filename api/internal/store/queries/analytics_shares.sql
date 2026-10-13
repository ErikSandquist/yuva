-- name: ListAnalyticsShares :many
SELECT * FROM analytics_shares WHERE workspace_id = @workspace_id AND property_id = @property_id ORDER BY created_at DESC, id;

-- name: GetAnalyticsShare :one
SELECT * FROM analytics_shares WHERE workspace_id = @workspace_id AND property_id = @property_id AND id = @id;

-- name: CountAnalyticsShares :one
SELECT count(*) FROM analytics_shares WHERE workspace_id = @workspace_id AND property_id = @property_id;

-- name: CreateAnalyticsShare :one
INSERT INTO analytics_shares (id, workspace_id, property_id, name, token_hash, token_prefix, views, password_hash, embed_origins,
    expires_at, created_by, created_at, updated_at)
VALUES (@id, @workspace_id, @property_id, @name, @token_hash, @token_prefix, @views, sqlc.narg(password_hash), @embed_origins,
    sqlc.narg(expires_at), sqlc.narg(created_by), @now, @now)
RETURNING *;

-- name: UpdateAnalyticsShare :one
UPDATE analytics_shares SET name = @name, views = @views, password_hash = sqlc.narg(password_hash), embed_origins = @embed_origins,
    expires_at = sqlc.narg(expires_at), updated_at = @now
WHERE workspace_id = @workspace_id AND property_id = @property_id AND id = @id
RETURNING *;

-- name: RevokeAnalyticsShare :one
UPDATE analytics_shares SET revoked_at = coalesce(revoked_at, sqlc.arg(now)::timestamptz), updated_at = CASE WHEN revoked_at IS NULL THEN sqlc.arg(now)::timestamptz ELSE updated_at END
WHERE workspace_id = @workspace_id AND property_id = @property_id AND id = @id
RETURNING *;

-- name: DeleteAnalyticsShare :execrows
DELETE FROM analytics_shares WHERE workspace_id = @workspace_id AND property_id = @property_id AND id = @id;

-- name: GetAnalyticsShareByTokenHash :one
-- The share page names only its token; everything after this lookup is scoped by the workspace it returns.
SELECT s.* FROM analytics_shares s
JOIN analytics_properties p ON p.workspace_id = s.workspace_id AND p.id = s.property_id AND p.deleted_at IS NULL
JOIN workspaces w ON w.id = s.workspace_id AND w.deleted_at IS NULL
WHERE s.token_hash = @token_hash AND s.revoked_at IS NULL AND (s.expires_at IS NULL OR s.expires_at > sqlc.arg(now)::timestamptz);

-- name: TouchAnalyticsShare :exec
UPDATE analytics_shares SET last_viewed_at = sqlc.arg(now)::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND (last_viewed_at IS NULL OR last_viewed_at < sqlc.arg(stale_before)::timestamptz);

-- name: ListAnalyticsExports :many
SELECT * FROM analytics_exports
WHERE workspace_id = @workspace_id AND property_id = @property_id AND expires_at > @now AND (sqlc.arg(with_raw)::boolean OR kind <> 'raw')
ORDER BY created_at DESC, id
LIMIT 200;

-- name: GetAnalyticsExport :one
SELECT * FROM analytics_exports WHERE workspace_id = @workspace_id AND property_id = @property_id AND id = @id AND expires_at > @now;

-- name: GetAnalyticsExportForJob :one
SELECT * FROM analytics_exports WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountUnfinishedAnalyticsExports :one
SELECT count(*) FROM analytics_exports WHERE workspace_id = @workspace_id AND status IN ('queued', 'running') AND expires_at > @now;

-- name: CreateAnalyticsExport :one
INSERT INTO analytics_exports (id, workspace_id, property_id, kind, format, params, from_day, to_day, created_by, created_at, expires_at)
VALUES (@id, @workspace_id, @property_id, @kind, @format, @params, @from_day, @to_day, sqlc.narg(created_by), @now, @expires_at)
RETURNING *;

-- name: StartAnalyticsExport :execrows
UPDATE analytics_exports SET status = 'running', progress = 0, rows = 0, size_bytes = 0, error = NULL
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('queued', 'running');

-- name: ProgressAnalyticsExport :execrows
UPDATE analytics_exports SET progress = @progress, rows = @rows
WHERE workspace_id = @workspace_id AND id = @id AND status = 'running';

-- name: FinishAnalyticsExport :execrows
UPDATE analytics_exports SET status = 'done', progress = 100, rows = @rows, size_bytes = @size_bytes, storage_key = @storage_key,
    finished_at = sqlc.arg(now)::timestamptz, expires_at = @expires_at
WHERE workspace_id = @workspace_id AND id = @id AND status = 'running';

-- name: FailAnalyticsExport :execrows
UPDATE analytics_exports SET status = 'failed', error = sqlc.arg(error)::text, finished_at = sqlc.arg(now)::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('queued', 'running');

-- name: DeleteAnalyticsExport :one
DELETE FROM analytics_exports WHERE workspace_id = @workspace_id AND property_id = @property_id AND id = @id
RETURNING storage_key;

-- name: ListExpiredAnalyticsExports :many
SELECT id, storage_key FROM analytics_exports WHERE workspace_id = @workspace_id AND expires_at <= @now ORDER BY expires_at LIMIT 500;

-- name: ListPropertyAnalyticsExports :many
SELECT id, storage_key FROM analytics_exports WHERE workspace_id = @workspace_id AND property_id = @property_id;

-- name: DeleteAnalyticsExportRow :exec
DELETE FROM analytics_exports WHERE workspace_id = @workspace_id AND id = @id;
