-- name: StatusDomainTaken :one
-- The documented exception: own domains are unique across the server, so availability is checked across workspaces.
SELECT EXISTS (SELECT 1 FROM status_pages WHERE custom_domain = @domain AND id <> @id)::bool;

-- name: CountStatusDomains :one
SELECT count(*) FROM status_pages
WHERE workspace_id = @workspace_id AND id <> @id AND custom_domain IS NOT NULL AND domain_state <> 'off';

-- name: SetStatusPageDomain :one
UPDATE status_pages SET custom_domain = sqlc.narg(domain), domain_token = sqlc.narg(token), domain_state = sqlc.narg(state),
    domain_problem = NULL, domain_checked_at = NULL, domain_check_at = sqlc.narg(check_at), domain_verified_at = NULL,
    domain_missing_since = NULL, cf_hostname_id = NULL, cf_status = NULL, cf_ssl_status = NULL, cf_errors = NULL, cf_checked_at = NULL,
    changed_at = greatest(changed_at + interval '1 microsecond', @now::timestamptz), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: CheckStatusDomainSoon :one
UPDATE status_pages SET domain_check_at = @now::timestamptz, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ListDueStatusDomains :many
-- The documented exception: the sweep lists the own domains due for a check of every workspace.
SELECT p.workspace_id, p.id FROM status_pages p
JOIN workspaces w ON w.id = p.workspace_id AND w.deleted_at IS NULL
WHERE p.domain_check_at <= @now::timestamptz AND p.domain_state IN ('pending', 'failed', 'verified')
ORDER BY p.domain_check_at
LIMIT @lim;

-- name: RecordStatusDomainCheck :one
UPDATE status_pages SET domain_state = @state::text, domain_problem = sqlc.narg(problem), domain_checked_at = @now::timestamptz,
    domain_check_at = sqlc.narg(check_at), domain_verified_at = sqlc.narg(verified_at), domain_missing_since = sqlc.narg(missing_since),
    changed_at = CASE WHEN @moved::bool THEN greatest(changed_at + interval '1 microsecond', @now::timestamptz) ELSE changed_at END,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetStatusPageCloudflare :one
UPDATE status_pages SET cf_hostname_id = sqlc.narg(hostname_id), cf_status = sqlc.narg(status), cf_ssl_status = sqlc.narg(ssl_status),
    cf_errors = sqlc.narg(errors)::text[], cf_checked_at = @now::timestamptz, domain_check_at = coalesce(sqlc.narg(check_at), domain_check_at), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FindStatusPageByDomain :one
-- The documented exception: a request on an own domain finds its page before its workspace is known.
SELECT * FROM status_pages WHERE custom_domain = @domain AND domain_state = 'verified';

-- name: ListStatusCloudflareHostnames :many
SELECT cf_hostname_id::text AS hostname_id, custom_domain::text AS domain FROM status_pages
WHERE workspace_id = @workspace_id AND cf_hostname_id IS NOT NULL;
