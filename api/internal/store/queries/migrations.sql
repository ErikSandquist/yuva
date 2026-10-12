-- name: CreateMigration :one
INSERT INTO migrations (id, workspace_id, source, preset, source_address, inbox_id, channel_id, options,
    created_by_member_id, created_by_api_key_id, created_via, created_at, updated_at)
VALUES (@id, @workspace_id, @source, @preset, @source_address, sqlc.narg(inbox_id), sqlc.narg(channel_id), @options,
    sqlc.narg(member_id), sqlc.narg(api_key_id), sqlc.narg(via), @now, @now)
RETURNING *;

-- name: GetMigration :one
SELECT * FROM migrations WHERE workspace_id = $1 AND id = $2;

-- name: LockMigration :one
SELECT * FROM migrations WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: ListMigrations :many
SELECT * FROM migrations
WHERE workspace_id = @workspace_id
  AND (cardinality(@states::text[]) = 0 OR state = ANY(@states::text[]))
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: UpdateMigration :one
UPDATE migrations SET preset = @preset, source_address = @source_address, inbox_id = sqlc.narg(inbox_id),
    channel_id = sqlc.narg(channel_id), options = @options, state = @state, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetMigrationState :one
UPDATE migrations SET state = @state, state_reason = @state_reason, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteMigration :execrows
DELETE FROM migrations WHERE workspace_id = $1 AND id = $2;

-- name: ListMigrationSources :many
SELECT id, source, preset FROM migrations WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);

-- name: SetMigrationCredentials :exec
INSERT INTO migration_credentials (workspace_id, migration_id, kind, sealed, host, port, tls, certificate_sha256,
    scopes, expires_at, created_at, updated_at)
VALUES (@workspace_id, @migration_id, @kind, @sealed, @host, @port, @tls, sqlc.narg(certificate_sha256),
    @scopes, sqlc.narg(expires_at), @now, @now)
ON CONFLICT (workspace_id, migration_id) DO UPDATE SET
    kind = excluded.kind, sealed = excluded.sealed, host = excluded.host, port = excluded.port, tls = excluded.tls,
    certificate_sha256 = excluded.certificate_sha256, scopes = excluded.scopes, expires_at = excluded.expires_at,
    updated_at = excluded.updated_at;

-- name: GetMigrationCredentials :one
SELECT * FROM migration_credentials WHERE workspace_id = $1 AND migration_id = $2;

-- name: ListMigrationCredentialInfos :many
SELECT migration_id, kind, host, port, tls, certificate_sha256, scopes, expires_at, updated_at
FROM migration_credentials
WHERE workspace_id = @workspace_id AND migration_id = ANY(@migration_ids::uuid[]);

-- name: AddMigrationAudit :one
INSERT INTO migration_audit (workspace_id, migration_id, action, actor_member_id, actor_api_key_id, via, detail, created_at)
VALUES (@workspace_id, @migration_id, @action, sqlc.narg(member_id), sqlc.narg(api_key_id), sqlc.narg(via), @detail, @now)
RETURNING id;

-- name: SetMigrationAuditDetail :exec
UPDATE migration_audit SET detail = @detail WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountMigrationTests :one
SELECT count(*) FILTER (WHERE migration_id = @migration_id)::int AS migration,
       count(*)::int AS workspace,
       coalesce(min(created_at) FILTER (WHERE migration_id = @migration_id), @since)::timestamptz AS migration_oldest,
       coalesce(min(created_at), @since)::timestamptz AS workspace_oldest
FROM migration_audit
WHERE workspace_id = @workspace_id AND action = 'tested' AND created_at > @since;

-- name: ListRecentMigrationTests :many
SELECT coalesce(detail->>'code', '')::text AS code, created_at FROM migration_audit
WHERE workspace_id = @workspace_id AND migration_id = @migration_id AND action = 'tested'
ORDER BY id DESC
LIMIT @lim;
