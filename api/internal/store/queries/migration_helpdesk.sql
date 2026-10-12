-- name: SetMigrationSignIn :exec
INSERT INTO migration_sign_ins (workspace_id, migration_id, state_hash, sealed, host, member_id, api_key_id, expires_at, created_at)
VALUES (@workspace_id, @migration_id, @state_hash, @sealed, @host, sqlc.narg(member_id), sqlc.narg(api_key_id), @expires_at, @now)
ON CONFLICT (workspace_id, migration_id) DO UPDATE SET
    state_hash = excluded.state_hash, sealed = excluded.sealed, host = excluded.host, member_id = excluded.member_id,
    api_key_id = excluded.api_key_id, expires_at = excluded.expires_at, created_at = excluded.created_at;

-- name: TakeMigrationSignIn :one
DELETE FROM migration_sign_ins
WHERE workspace_id = @workspace_id AND migration_id = @migration_id AND state_hash = @state_hash AND expires_at > @now
RETURNING *;

-- name: DeleteMigrationSignIn :exec
DELETE FROM migration_sign_ins WHERE workspace_id = @workspace_id AND migration_id = @migration_id;

-- name: SetMigrationSourceState :exec
UPDATE migrations SET source_state = @source_state WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetMigrationFolderSource :exec
UPDATE migration_folders SET source_ref = @source_ref, address = sqlc.narg(address)
WHERE workspace_id = @workspace_id AND migration_id = @migration_id AND name = @name;

-- A ticket the scan or sync reads again takes its new details unless it was imported already.
-- name: UpsertHelpdeskItems :exec
INSERT INTO migration_items (workspace_id, migration_id, source_id, folder_id, thread_key, dedupe_key, received_at, size_bytes,
    meta, state, reason, in_copy)
SELECT @workspace_id, @migration_id, unnest(@source_ids::text[]), nullif(unnest(@folder_ids::int[]), 0), unnest(@source_ids::text[]),
    unnest(@source_ids::text[]), unnest(@received_ats::timestamptz[]), 0, unnest(@metas::jsonb[]), 'skipped', 'unplanned', false
ON CONFLICT (workspace_id, migration_id, source_id) DO UPDATE SET
    folder_id = excluded.folder_id, received_at = excluded.received_at, meta = excluded.meta
WHERE migration_items.state <> 'imported';

-- name: ListHelpdeskItems :many
SELECT source_id, folder_id, meta, state FROM migration_items
WHERE workspace_id = @workspace_id AND migration_id = @migration_id AND source_id > @after
ORDER BY source_id COLLATE "C"
LIMIT @lim;

-- name: GetHelpdeskItem :one
SELECT source_id, folder_id, meta, state, received_at FROM migration_items
WHERE workspace_id = @workspace_id AND migration_id = @migration_id AND source_id = @source_id
FOR UPDATE;

-- A ticket that changed in the help desk is copied again; messages already imported stay as they are.
-- name: ReplanHelpdeskItem :exec
UPDATE migration_items SET meta = @meta, received_at = @received_at, folder_id = nullif(@folder_id::int, 0), state = 'planned', reason = '',
    attempts = 0, in_copy = true, thread_key = source_id
WHERE workspace_id = @workspace_id AND migration_id = @migration_id AND source_id = @source_id;

-- name: SkipHelpdeskItem :exec
UPDATE migration_items SET meta = @meta, received_at = @received_at, folder_id = nullif(@folder_id::int, 0), state = 'skipped', reason = @reason,
    in_copy = true, thread_key = source_id
WHERE workspace_id = @workspace_id AND migration_id = @migration_id AND source_id = @source_id AND state = 'planned';

-- name: ReopenHelpdeskThread :exec
INSERT INTO migration_threads (workspace_id, migration_id, thread_key, last_at, items, done, plan)
VALUES (@workspace_id, @migration_id, @thread_key, @last_at, 1, 0, @plan)
ON CONFLICT (workspace_id, migration_id, thread_key) DO UPDATE SET
    last_at = greatest(migration_threads.last_at, excluded.last_at), items = 1, done = 0, plan = excluded.plan;

-- name: CountHelpdeskPlanContacts :one
WITH r AS (
    SELECT DISTINCT coalesce(nullif(meta->'requester'->>'email', ''), '#' || (meta->'requester'->>'id')) AS who
    FROM migration_items
    WHERE workspace_id = @workspace_id AND migration_id = @migration_id AND state = 'planned')
SELECT count(*)::bigint AS total, count(ce.email)::bigint AS existing
FROM r LEFT JOIN contact_emails ce ON ce.workspace_id = @workspace_id AND ce.email = r.who;

-- name: ListHelpdeskAssignees :many
SELECT (meta->'assignee'->>'email')::text AS email, max(meta->'assignee'->>'name')::text AS name, count(*)::bigint AS conversations
FROM migration_items
WHERE workspace_id = @workspace_id AND migration_id = @migration_id AND state = 'planned'
  AND coalesce(meta->'assignee'->>'email', '') <> ''
GROUP BY 1;

-- name: ImportedMessageExists :one
SELECT EXISTS (SELECT 1 FROM messages WHERE workspace_id = @workspace_id AND id = @id)::bool;

-- name: ImportNote :exec
INSERT INTO messages (id, workspace_id, conversation_id, kind, author_type, author_member_id, body, html, created_at,
    imported_at, imported_author)
VALUES (@id, @workspace_id, @conversation_id, 'note', 'member', sqlc.narg(author_member_id), @body, sqlc.narg(html), @created_at,
    @imported_at, sqlc.narg(imported_author));

-- name: SetImportedConversationAssignee :exec
UPDATE conversations SET assignee_id = @assignee_id WHERE workspace_id = @workspace_id AND id = @id;

-- Attributes a contact already has keep their value.
-- name: MergeImportedContactAttributes :exec
UPDATE contacts SET attributes = @attributes::jsonb || attributes WHERE workspace_id = @workspace_id AND id = @id;
