-- name: StatusDomainTaken :one
-- The documented exception: a verified own domain is unique across the server, so availability is checked across workspaces.
SELECT EXISTS (SELECT 1 FROM status_pages WHERE custom_domain = @domain AND id <> @id AND domain_state = 'verified')::bool;

-- name: LockStatusDomainName :exec
SELECT pg_advisory_xact_lock(hashtextextended('yuva_status_domain:' || @domain::text, 0));

-- name: LockStatusDomainCount :exec
SELECT pg_advisory_xact_lock(hashtextextended('yuva_status_domains:' || sqlc.arg(workspace_id)::uuid::text, 0));

-- name: FailStatusDomainClaims :many
-- The documented exception: once a page verifies a domain, the other claims of every workspace lose it.
UPDATE status_pages SET domain_state = 'failed', domain_problem = 'taken', domain_check_at = NULL, updated_at = @now
WHERE custom_domain = @domain AND id <> @id AND domain_state IN ('pending', 'failed')
RETURNING id, workspace_id, inbox_id;

-- name: CountStatusDomains :one
SELECT count(*) FROM status_pages
WHERE workspace_id = @workspace_id AND id <> @id AND custom_domain IS NOT NULL AND domain_state <> 'off';

-- name: SetStatusPageDomain :one
UPDATE status_pages SET custom_domain = sqlc.narg(domain), domain_token = sqlc.narg(token), domain_state = sqlc.narg(state),
    domain_problem = NULL, domain_checked_at = NULL, domain_check_at = sqlc.narg(check_at), domain_verified_at = NULL,
    domain_missing_since = NULL, domain_claimed_at = CASE WHEN sqlc.narg(domain)::text IS NULL THEN NULL ELSE @now::timestamptz END, cf_hostname_id = NULL, cf_status = NULL, cf_ssl_status = NULL, cf_errors = NULL, cf_checked_at = NULL,
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
SELECT p.* FROM status_pages p
JOIN workspaces w ON w.id = p.workspace_id AND w.deleted_at IS NULL
WHERE p.custom_domain = @domain AND p.domain_state = 'verified';

-- name: ListStatusCloudflareHostnames :many
SELECT cf_hostname_id::text AS hostname_id, custom_domain::text AS domain FROM status_pages
WHERE workspace_id = @workspace_id AND cf_hostname_id IS NOT NULL;
