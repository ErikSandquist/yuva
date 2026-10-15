-- name: HomeInbox :one
WITH open AS (
    SELECT c.id, c.inbox_id, c.subject, c.contact_id, c.assignee_id
    FROM conversations c
    WHERE c.workspace_id = @workspace_id AND c.status = 'open' AND NOT c.spam
      AND (@all_inboxes::bool OR EXISTS (
          SELECT 1 FROM inbox_viewers iv
          WHERE iv.workspace_id = c.workspace_id AND iv.inbox_id = c.inbox_id AND iv.viewer_id = @viewer_id::uuid))
), waiting AS (
    SELECT o.id, o.inbox_id, o.subject, o.contact_id, w.since
    FROM open o
    CROSS JOIN LATERAL (
        SELECT min(m.created_at)::timestamptz AS since FROM messages m
        WHERE m.workspace_id = @workspace_id AND m.conversation_id = o.id AND m.kind = 'message' AND m.direction = 'in'
          AND m.created_at > coalesce((
              SELECT max(r.created_at) FROM messages r
              WHERE r.workspace_id = @workspace_id AND r.conversation_id = o.id AND r.kind = 'message' AND r.direction = 'out'
                AND NOT r.draft AND r.author_type IN ('member', 'bot')), '-infinity'::timestamptz)
    ) w
    WHERE w.since IS NOT NULL
), oldest AS (
    SELECT * FROM waiting ORDER BY since, id LIMIT 1
)
SELECT n.open, n.unassigned, n.waiting, o.id AS oldest_id, o.inbox_id AS oldest_inbox_id, o.subject AS oldest_subject,
    o.since AS oldest_since, ct.id AS oldest_contact_id, ct.name AS oldest_contact_name, ce.email AS oldest_contact_email
FROM (SELECT (SELECT count(*) FROM open)::bigint AS open,
             (SELECT count(*) FROM open WHERE assignee_id IS NULL)::bigint AS unassigned,
             (SELECT count(*) FROM waiting)::bigint AS waiting) n
LEFT JOIN oldest o ON true
LEFT JOIN contacts ct ON ct.workspace_id = @workspace_id AND ct.id = o.contact_id
LEFT JOIN contact_emails ce ON ce.workspace_id = @workspace_id AND ce.contact_id = o.contact_id
    AND ce.position = (SELECT min(e.position) FROM contact_emails e WHERE e.workspace_id = @workspace_id AND e.contact_id = o.contact_id);

-- name: HomeLastBroadcast :one
SELECT * FROM broadcasts b
WHERE b.workspace_id = @workspace_id AND b.state = 'sent'
  AND (@all_inboxes::bool OR EXISTS (
      SELECT 1 FROM inbox_viewers iv
      WHERE iv.workspace_id = b.workspace_id AND iv.inbox_id = b.inbox_id AND iv.viewer_id = @viewer_id::uuid))
ORDER BY b.finished_at DESC NULLS LAST, b.id DESC
LIMIT 1;

-- name: ListSetupParts :many
WITH visible AS (
    SELECT i.id, i.name, i.created_at FROM inboxes i
    WHERE i.workspace_id = @workspace_id
      AND (@all_inboxes::bool OR EXISTS (
          SELECT 1 FROM inbox_viewers iv
          WHERE iv.workspace_id = i.workspace_id AND iv.inbox_id = i.id AND iv.viewer_id = @viewer_id::uuid))
)
SELECT 'inbox'::text AS part, v.id, NULL::uuid AS inbox_id, v.name, ''::text AS kind, v.created_at,
    '{}'::text[] AS domains, '{}'::text[] AS platforms, false AS docs,
    row_number() OVER (ORDER BY v.name, v.id) AS pos
FROM visible v
WHERE @with_inboxes::bool
UNION ALL
SELECT 'channel', ch.id, ch.inbox_id, ch.name, ch.kind, ch.created_at, '{}', '{}',
    ch.kind = 'chat' AND (
        EXISTS (SELECT 1 FROM page_ratings r WHERE r.workspace_id = ch.workspace_id AND r.channel_id = ch.id)
        OR EXISTS (SELECT 1 FROM conversations c WHERE c.workspace_id = ch.workspace_id AND c.channel_id = ch.id AND c.page_url IS NOT NULL)
        OR EXISTS (SELECT 1 FROM discussions d WHERE d.workspace_id = ch.workspace_id AND d.inbox_id = ch.inbox_id AND d.channel_id = ch.id)),
    row_number() OVER (ORDER BY ch.created_at, ch.id)
FROM channels ch
WHERE @with_inboxes::bool AND @with_channels::bool AND ch.workspace_id = @workspace_id AND ch.inbox_id IN (SELECT id FROM visible)
UNION ALL
SELECT 'property', p.id, p.inbox_id, p.name, '', p.created_at, p.domains, p.platforms, false,
    row_number() OVER (ORDER BY p.created_at, p.id)
FROM analytics_properties p
WHERE @with_properties::bool AND p.workspace_id = @workspace_id AND p.deleted_at IS NULL
  AND (@all_inboxes::bool OR EXISTS (
      SELECT 1 FROM inbox_viewers iv
      WHERE iv.workspace_id = p.workspace_id AND iv.inbox_id = p.inbox_id AND iv.viewer_id = @viewer_id::uuid))
UNION ALL
SELECT 'list', l.id, l.inbox_id, l.name, '', l.created_at, '{}', '{}', false,
    row_number() OVER (ORDER BY l.created_at DESC, l.id DESC)
FROM mailing_lists l
WHERE @with_lists::bool AND l.workspace_id = @workspace_id AND l.archived_at IS NULL
  AND (@all_inboxes::bool OR EXISTS (
      SELECT 1 FROM inbox_viewers iv
      WHERE iv.workspace_id = l.workspace_id AND iv.inbox_id = l.inbox_id AND iv.viewer_id = @viewer_id::uuid))
ORDER BY 1, 10;
