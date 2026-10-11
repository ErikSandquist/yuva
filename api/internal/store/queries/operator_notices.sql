-- name: CreateOperatorNotice :exec
INSERT INTO operator_notices (id, kind, email, reader, workspace_name, ip, country, detail, created_at)
VALUES (@id, @kind, @email, @reader, sqlc.narg(workspace_name), @ip, sqlc.narg(country), sqlc.narg(detail), @created_at);

-- name: LockOperatorNotices :exec
SELECT pg_advisory_xact_lock(hashtext('yuva.operator_notices'));

-- name: LastOperatorDigestAt :one
SELECT sent_at FROM operator_notices WHERE sent_at IS NOT NULL ORDER BY sent_at DESC LIMIT 1;

-- name: ListUnsentOperatorNotices :many
SELECT * FROM operator_notices WHERE sent_at IS NULL ORDER BY created_at, id;

-- name: MarkOperatorNoticesSent :exec
UPDATE operator_notices SET sent_at = @sent_at::timestamptz WHERE id = ANY(@ids::uuid[]);

-- name: DeleteSentOperatorNotices :exec
DELETE FROM operator_notices WHERE sent_at < @before::timestamptz;
