-- name: CreateListImport :one
INSERT INTO list_imports (id, workspace_id, list_id, member_id, api_key_id, state, filename, storage_key, columns,
    attestation, consent_source, reconfirm, created_at, updated_at)
VALUES (@id, @workspace_id, @list_id, sqlc.narg(member_id), sqlc.narg(api_key_id), 'validating', @filename, @storage_key,
    @columns, @attestation, @consent_source, @reconfirm, @now, @now)
RETURNING *;

-- name: GetListImport :one
SELECT * FROM list_imports WHERE workspace_id = $1 AND id = $2;

-- name: LockListImport :one
SELECT * FROM list_imports WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: ListListImports :many
SELECT * FROM list_imports
WHERE workspace_id = @workspace_id AND list_id = @list_id
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: RevalidateListImport :one
UPDATE list_imports SET state = 'validating', columns = @columns, preview = NULL, counts = '{}', rows_total = 0,
    error = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SaveListImportDomains :exec
UPDATE list_imports SET domains = @domains WHERE workspace_id = @workspace_id AND id = @id;

-- name: SaveListImportValidation :one
UPDATE list_imports SET state = 'ready', encoding = @encoding, delimiter = @delimiter, columns = @columns, preview = @preview,
    rows_total = @rows_total, counts = @counts, domains = @domains, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND state = 'validating'
RETURNING *;

-- name: StartListImport :one
UPDATE list_imports SET state = 'importing', started_at = @now, rows_done = 0, counts = @counts, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND state = 'ready'
RETURNING *;

-- name: SaveListImportProgress :one
UPDATE list_imports SET rows_done = @rows_done, counts = @counts, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FinishListImport :one
UPDATE list_imports SET state = @state, error = sqlc.narg(error), storage_key = NULL, preview = NULL, finished_at = @now,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: AddListImportErrors :exec
INSERT INTO list_import_errors (workspace_id, import_id, row, email, reason)
SELECT @workspace_id, @import_id, unnest(@rows::int[]), unnest(@emails::text[]), unnest(@reasons::text[])
ON CONFLICT DO NOTHING;

-- name: ListListImportErrors :many
SELECT row, email, reason FROM list_import_errors
WHERE workspace_id = @workspace_id AND import_id = @import_id AND row > @after_row
ORDER BY row
LIMIT @lim;

-- name: DeleteExpiredListImportErrors :execrows
DELETE FROM list_import_errors e
USING list_imports i
WHERE e.workspace_id = @workspace_id AND i.workspace_id = e.workspace_id AND i.id = e.import_id
  AND i.finished_at < @before::timestamptz;

-- name: CancelStaleListImports :many
WITH stale AS (
    SELECT id, storage_key FROM list_imports
    WHERE list_imports.workspace_id = @workspace_id::uuid AND state IN ('validating', 'ready') AND updated_at < @before::timestamptz
    FOR UPDATE
), done AS (
    UPDATE list_imports i SET state = 'cancelled', storage_key = NULL, preview = NULL, finished_at = @now, updated_at = @now
    FROM stale s
    WHERE i.workspace_id = @workspace_id::uuid AND i.id = s.id
)
SELECT storage_key::text FROM stale WHERE storage_key IS NOT NULL;

-- name: ListImportStorageKeys :many
SELECT storage_key::text FROM list_imports
WHERE workspace_id = @workspace_id AND storage_key IS NOT NULL
  AND (sqlc.narg(list_id)::uuid IS NULL OR list_id = sqlc.narg(list_id)::uuid);

-- name: SuppressedEmailsIn :many
SELECT DISTINCT email FROM email_suppressions WHERE workspace_id = @workspace_id AND email = ANY(@emails::text[]) AND scope <> 'confirmations';

-- name: OptedOutHashesIn :many
SELECT email_hash FROM email_optouts WHERE workspace_id = @workspace_id AND email_hash = ANY(@hashes::bytea[]);

-- name: ListSubscriptionsByEmails :many
SELECT email, state, unsubscribe_reason FROM list_subscriptions
WHERE workspace_id = @workspace_id AND list_id = @list_id AND email = ANY(@emails::text[]);
