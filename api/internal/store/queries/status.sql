-- name: CreateStatusPage :one
INSERT INTO status_pages (id, workspace_id, inbox_id, name, description, slug, logo_url, accent_color, site_url, support_url,
    locale, timezone, history_days, show_uptime, published, changed_at, created_at, updated_at)
VALUES (@id, @workspace_id, @inbox_id, @name, @description, @slug, sqlc.narg(logo_url), sqlc.narg(accent_color), sqlc.narg(site_url),
    sqlc.narg(support_url), @locale, @timezone, @history_days, @show_uptime, @published, @now, @now, @now)
RETURNING *;

-- name: GetStatusPage :one
SELECT * FROM status_pages WHERE workspace_id = $1 AND id = $2;

-- name: LockStatusPage :one
SELECT * FROM status_pages WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: GetInboxStatusPage :one
SELECT * FROM status_pages WHERE workspace_id = $1 AND inbox_id = $2;

-- name: StatusSlugTaken :one
SELECT EXISTS (SELECT 1 FROM status_pages WHERE slug = @slug AND id <> @id)::bool;

-- name: FindStatusPageBySlug :one
-- The documented exception: a public page is found by its slug before its workspace is known.
SELECT * FROM status_pages WHERE slug = $1;

-- name: UpdateStatusPage :one
UPDATE status_pages SET name = @name, description = @description, slug = @slug, logo_url = sqlc.narg(logo_url),
    accent_color = sqlc.narg(accent_color), site_url = sqlc.narg(site_url), support_url = sqlc.narg(support_url),
    locale = @locale, timezone = @timezone, history_days = @history_days, show_uptime = @show_uptime, published = @published,
    changed_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: TouchStatusPage :exec
UPDATE status_pages SET changed_at = greatest(changed_at + interval '1 microsecond', @now::timestamptz)
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteStatusPage :execrows
DELETE FROM status_pages WHERE workspace_id = $1 AND id = $2;

-- name: ListStatusPages :many
SELECT * FROM status_pages p
WHERE p.workspace_id = @workspace_id
  AND (sqlc.narg(inbox_id)::uuid IS NULL OR p.inbox_id = sqlc.narg(inbox_id)::uuid)
  AND (@all_inboxes::bool OR EXISTS (
      SELECT 1 FROM inbox_viewers iv
      WHERE iv.workspace_id = p.workspace_id AND iv.inbox_id = p.inbox_id AND iv.viewer_id = @viewer_id))
ORDER BY p.created_at, p.id;

-- name: CreateStatusGroup :one
INSERT INTO status_component_groups (id, workspace_id, page_id, name, position, created_at, updated_at)
VALUES (@id, @workspace_id, @page_id, @name, @position, @now, @now)
RETURNING *;

-- name: GetStatusGroup :one
SELECT * FROM status_component_groups WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id;

-- name: UpdateStatusGroup :one
UPDATE status_component_groups SET name = @name, position = @position, updated_at = @now
WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id
RETURNING *;

-- name: DeleteStatusGroup :execrows
DELETE FROM status_component_groups WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id;

-- name: ListStatusGroups :many
SELECT * FROM status_component_groups WHERE workspace_id = @workspace_id AND page_id = @page_id ORDER BY position, name, id;

-- name: CountStatusComponents :one
SELECT count(*)::bigint FROM status_components WHERE workspace_id = @workspace_id AND page_id = @page_id;

-- name: CreateStatusComponent :one
INSERT INTO status_components (id, workspace_id, page_id, group_id, name, description, position, hidden, state, created_at, updated_at)
VALUES (@id, @workspace_id, @page_id, sqlc.narg(group_id), @name, @description, @position, @hidden, @state, @now, @now)
RETURNING *;

-- name: GetStatusComponent :one
SELECT * FROM status_components WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id;

-- name: LockStatusComponent :one
SELECT * FROM status_components WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id FOR UPDATE;

-- name: UpdateStatusComponent :one
UPDATE status_components SET group_id = sqlc.narg(group_id), name = @name, description = @description, position = @position,
    hidden = @hidden, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetStatusComponentState :one
UPDATE status_components SET state = @state, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteStatusComponent :execrows
DELETE FROM status_components WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id;

-- name: ListStatusComponents :many
SELECT c.* FROM status_components c
LEFT JOIN status_component_groups g ON g.workspace_id = c.workspace_id AND g.id = c.group_id
WHERE c.workspace_id = @workspace_id AND c.page_id = @page_id
ORDER BY g.position NULLS FIRST, g.name NULLS FIRST, g.id NULLS FIRST, c.position, c.name, c.id;

-- name: ListStatusComponentsByIDs :many
SELECT * FROM status_components WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = ANY(@ids::uuid[]);

-- name: EndStatusComponentState :exec
UPDATE status_component_states SET ended_at = greatest(started_at, @now::timestamptz)
WHERE workspace_id = @workspace_id AND component_id = @component_id AND ended_at IS NULL;

-- name: StartStatusComponentState :one
INSERT INTO status_component_states (id, workspace_id, component_id, state, source, source_id, started_at)
VALUES (@id, @workspace_id, @component_id, @state, @source, sqlc.narg(source_id), @now)
RETURNING *;

-- name: OpenStatusComponentStates :many
SELECT * FROM status_component_states
WHERE workspace_id = @workspace_id AND component_id = ANY(@component_ids::uuid[]) AND ended_at IS NULL;

-- name: StatusComponentSpans :many
SELECT * FROM status_component_states
WHERE workspace_id = @workspace_id AND component_id = ANY(@component_ids::uuid[])
  AND (ended_at IS NULL OR ended_at > @since::timestamptz)
ORDER BY component_id, started_at;

-- name: CreateStatusIncident :one
INSERT INTO status_incidents (id, workspace_id, page_id, kind, title, impact, state, scheduled_start, scheduled_end,
    published_at, started_at, created_by, created_at, updated_at)
VALUES (@id, @workspace_id, @page_id, @kind, @title, @impact, @state, sqlc.narg(scheduled_start), sqlc.narg(scheduled_end),
    sqlc.narg(published_at), sqlc.narg(started_at), sqlc.narg(created_by), @now, @now)
RETURNING *;

-- name: GetStatusIncident :one
SELECT * FROM status_incidents WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id;

-- name: LockStatusIncident :one
SELECT * FROM status_incidents WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id FOR UPDATE;

-- name: UpdateStatusIncident :one
UPDATE status_incidents SET title = @title, impact = @impact, state = @state, scheduled_start = sqlc.narg(scheduled_start),
    scheduled_end = sqlc.narg(scheduled_end), published_at = sqlc.narg(published_at), started_at = sqlc.narg(started_at),
    resolved_at = sqlc.narg(resolved_at), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteStatusIncident :execrows
DELETE FROM status_incidents WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id AND published_at IS NULL;

-- name: ListStatusIncidents :many
SELECT * FROM status_incidents
WHERE workspace_id = @workspace_id AND page_id = @page_id
  AND (sqlc.narg(kind)::text IS NULL OR kind = sqlc.narg(kind)::text)
  AND (NOT @open_only::bool OR resolved_at IS NULL)
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: ListPublicStatusIncidents :many
SELECT * FROM status_incidents
WHERE workspace_id = @workspace_id AND page_id = @page_id AND published_at IS NOT NULL
  AND (resolved_at IS NULL OR resolved_at >= @since::timestamptz)
ORDER BY coalesce(scheduled_start, published_at) DESC, id DESC
LIMIT @lim;

-- name: ListStatusIncidentsByIDs :many
SELECT * FROM status_incidents WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);

-- name: ClearStatusIncidentComponents :exec
DELETE FROM status_incident_components WHERE workspace_id = @workspace_id AND incident_id = @incident_id;

-- name: AddStatusIncidentComponent :exec
INSERT INTO status_incident_components (workspace_id, incident_id, component_id, state)
VALUES (@workspace_id, @incident_id, @component_id, @state);

-- name: ListStatusIncidentComponents :many
SELECT * FROM status_incident_components WHERE workspace_id = @workspace_id AND incident_id = ANY(@incident_ids::uuid[])
ORDER BY incident_id, component_id;

-- name: ActiveStatusHolders :many
SELECT ic.component_id, ic.state, i.id AS incident_id
FROM status_incident_components ic
JOIN status_incidents i ON i.workspace_id = ic.workspace_id AND i.id = ic.incident_id
WHERE ic.workspace_id = @workspace_id AND ic.component_id = ANY(@component_ids::uuid[])
  AND i.published_at IS NOT NULL AND i.state IN ('investigating', 'identified', 'monitoring', 'in_progress')
  AND i.id <> @except_id;

-- name: CreateStatusIncidentUpdate :one
INSERT INTO status_incident_updates (id, workspace_id, incident_id, state, body, author_member_id, author_key_id, created_at)
VALUES (@id, @workspace_id, @incident_id, @state, @body, sqlc.narg(author_member_id), sqlc.narg(author_key_id), @now)
RETURNING *;

-- name: EditStatusIncidentUpdate :one
UPDATE status_incident_updates SET body = @body, edited_at = sqlc.narg(edited_at)
WHERE workspace_id = @workspace_id AND incident_id = @incident_id AND id = @id
RETURNING *;

-- name: ListStatusIncidentUpdates :many
SELECT * FROM status_incident_updates WHERE workspace_id = @workspace_id AND incident_id = ANY(@incident_ids::uuid[])
ORDER BY created_at DESC, id DESC;
