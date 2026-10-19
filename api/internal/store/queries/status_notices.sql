-- name: LockPendingStatusNotice :one
SELECT * FROM status_notices
WHERE workspace_id = @workspace_id AND incident_id = @incident_id AND state = 'pending'
FOR UPDATE;

-- name: LastStartedStatusNotice :one
SELECT started_at::timestamptz FROM status_notices
WHERE workspace_id = @workspace_id AND incident_id = @incident_id AND started_at IS NOT NULL
ORDER BY started_at DESC
LIMIT 1;

-- name: CreateStatusNotice :one
INSERT INTO status_notices (id, workspace_id, page_id, incident_id, final, due_at, created_at, updated_at)
VALUES (@id, @workspace_id, @page_id, @incident_id, @final, @due_at, @now, @now)
RETURNING *;

-- name: MoveStatusNotice :one
UPDATE status_notices SET final = @final, due_at = @due_at, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: AttachStatusNoticeUpdate :exec
UPDATE status_incident_updates SET notice_id = @notice_id
WHERE workspace_id = @workspace_id AND id = @id;

-- name: GetStatusNotice :one
SELECT * FROM status_notices WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id;

-- name: LockStatusNotice :one
SELECT * FROM status_notices WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: ListStatusNotices :many
SELECT * FROM status_notices
WHERE workspace_id = @workspace_id AND page_id = @page_id
  AND (sqlc.narg(incident_id)::uuid IS NULL OR incident_id = sqlc.narg(incident_id))
  AND (sqlc.narg(state)::text IS NULL OR state = sqlc.narg(state))
  AND (sqlc.narg(before_at)::timestamptz IS NULL OR (created_at, id) < (sqlc.narg(before_at), sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: ListStatusNoticeUpdateIDs :many
SELECT notice_id::uuid AS notice_id, id FROM status_incident_updates
WHERE workspace_id = @workspace_id AND notice_id = ANY(@notice_ids::uuid[])
ORDER BY created_at DESC, id DESC;

-- name: ListStatusNoticeUpdates :many
SELECT * FROM status_incident_updates
WHERE workspace_id = @workspace_id AND notice_id = @notice_id
ORDER BY created_at DESC, id DESC;

-- name: SetStatusNoticeState :one
UPDATE status_notices SET state = @state, skip_reason = sqlc.narg(skip_reason), lease_until = sqlc.narg(lease_until),
    started_at = coalesce(started_at, sqlc.narg(started_at)), finished_at = sqlc.narg(finished_at), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: CancelStatusNotice :one
UPDATE status_notices SET state = 'cancelled', finished_at = @now::timestamptz, updated_at = @now
WHERE workspace_id = @workspace_id AND page_id = @page_id AND id = @id AND state = 'pending'
RETURNING *;

-- name: RenewStatusNoticeLease :exec
UPDATE status_notices SET lease_until = @lease_until, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND state = 'sending';

-- name: FinishStatusNoticeSnapshot :exec
UPDATE status_notices SET snapshot_done = true, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountStartedStatusNotices :one
SELECT count(*)::bigint FROM status_notices
WHERE workspace_id = @workspace_id AND page_id = @page_id AND state IN ('sending', 'sent') AND NOT final
  AND started_at > @since::timestamptz;

-- Sweeps run before a workspace is known; each row is then handled in its own workspace.
-- name: DueStatusNotices :many
SELECT workspace_id, id, due_at FROM status_notices
WHERE (state = 'pending' AND due_at <= @now::timestamptz) OR (state = 'sending' AND lease_until <= @now::timestamptz)
ORDER BY due_at
LIMIT 500;

-- A subscriber who chose components and has none left gets only incidents that touch no component.
-- name: StatusNoticeRecipients :many
SELECT s.id, s.contact_id, s.email FROM list_subscriptions s
WHERE s.workspace_id = @workspace_id AND s.list_id = @list_id AND s.state = 'subscribed' AND s.id > @after::uuid
  AND NOT EXISTS (SELECT 1 FROM email_suppressions x WHERE x.workspace_id = s.workspace_id AND x.email = s.email)
  AND (NOT s.chose_components
       OR EXISTS (SELECT 1 FROM status_subscriber_components c
                  JOIN status_incident_components ic ON ic.workspace_id = c.workspace_id AND ic.component_id = c.component_id
                  WHERE c.workspace_id = s.workspace_id AND c.subscription_id = s.id AND ic.incident_id = @incident_id)
       OR (NOT EXISTS (SELECT 1 FROM status_subscriber_components c WHERE c.workspace_id = s.workspace_id AND c.subscription_id = s.id)
           AND NOT EXISTS (SELECT 1 FROM status_incident_components ic WHERE ic.workspace_id = s.workspace_id AND ic.incident_id = @incident_id)))
  AND (NOT @earlier_only::bool OR EXISTS (
        SELECT 1 FROM status_notice_deliveries d
        JOIN status_notices n ON n.workspace_id = d.workspace_id AND n.id = d.notice_id
        WHERE d.workspace_id = s.workspace_id AND d.subscription_id = s.id AND n.incident_id = @incident_id
          AND d.notice_id <> @notice_id AND d.sent_at IS NOT NULL))
ORDER BY s.id
LIMIT @lim;

-- name: CountStatusNoticeReach :one
SELECT count(*)::bigint FROM list_subscriptions s
WHERE s.workspace_id = @workspace_id AND s.list_id = @list_id AND s.state = 'subscribed'
  AND NOT EXISTS (SELECT 1 FROM email_suppressions x WHERE x.workspace_id = s.workspace_id AND x.email = s.email)
  AND (NOT s.chose_components
       OR EXISTS (SELECT 1 FROM status_subscriber_components c
                  WHERE c.workspace_id = s.workspace_id AND c.subscription_id = s.id AND c.component_id = ANY(@component_ids::uuid[]))
       OR (NOT EXISTS (SELECT 1 FROM status_subscriber_components c WHERE c.workspace_id = s.workspace_id AND c.subscription_id = s.id)
           AND coalesce(cardinality(@component_ids::uuid[]), 0) = 0));

-- name: HasEarlierSentStatusNotice :one
SELECT EXISTS (
    SELECT 1 FROM status_notice_deliveries d
    JOIN status_notices n ON n.workspace_id = d.workspace_id AND n.id = d.notice_id
    WHERE d.workspace_id = @workspace_id AND n.incident_id = @incident_id AND d.notice_id <> @notice_id AND d.sent_at IS NOT NULL
) AS earlier;

-- name: InsertStatusNoticeDeliveries :execrows
INSERT INTO status_notice_deliveries (id, workspace_id, notice_id, subscription_id, contact_id, email, email_token, header_message_id,
    previous_message_id, created_at)
SELECT unnest(@ids::uuid[]), @workspace_id, @notice_id, unnest(@subscription_ids::uuid[]), unnest(@contact_ids::uuid[]),
       unnest(@emails::text[]), unnest(@tokens::text[]), unnest(@headers::text[]), nullif(unnest(@previous::text[]), ''), @now
ON CONFLICT DO NOTHING;

-- name: PreviousStatusNoticeMessages :many
SELECT d.subscription_id, d.header_message_id FROM status_notice_deliveries d
JOIN status_notices n ON n.workspace_id = d.workspace_id AND n.id = d.notice_id
WHERE d.workspace_id = @workspace_id AND n.incident_id = @incident_id AND d.notice_id <> @notice_id
  AND d.subscription_id = ANY(@subscription_ids::uuid[]) AND d.state IN ('sent', 'bounced', 'complained')
ORDER BY d.created_at DESC;

-- name: DueStatusNoticeDeliveries :many
SELECT * FROM status_notice_deliveries
WHERE workspace_id = @workspace_id AND notice_id = @notice_id AND state = 'queued'
  AND (next_attempt_at IS NULL OR next_attempt_at <= @now::timestamptz)
ORDER BY next_attempt_at NULLS FIRST, id
LIMIT @lim;

-- name: QueuedStatusNoticeDeliveries :one
SELECT count(*)::bigint AS queued,
       coalesce(CASE WHEN bool_or(next_attempt_at IS NULL) THEN NULL ELSE min(next_attempt_at) END, 'epoch')::timestamptz AS next_attempt
FROM status_notice_deliveries
WHERE workspace_id = @workspace_id AND notice_id = @notice_id AND state = 'queued';

-- name: SaveStatusNoticeDelivery :exec
UPDATE status_notice_deliveries SET state = @state, skip_reason = sqlc.narg(skip_reason), attempts = @attempts,
    next_attempt_at = sqlc.narg(next_attempt_at), error = sqlc.narg(error), sent_at = sqlc.narg(sent_at),
    bounced_at = sqlc.narg(bounced_at), relay_message_id = coalesce(sqlc.narg(relay_message_id), relay_message_id)
WHERE workspace_id = @workspace_id AND id = @id AND state = 'queued';

-- name: RecountStatusNotice :one
UPDATE status_notices n SET counts = (
    SELECT jsonb_build_object(
        'recipients', count(*), 'queued', count(*) FILTER (WHERE d.state = 'queued'),
        'sent', count(*) FILTER (WHERE d.state IN ('sent', 'bounced', 'complained') AND d.sent_at IS NOT NULL),
        'failed', count(*) FILTER (WHERE d.state = 'failed'), 'bounced', count(*) FILTER (WHERE d.state = 'bounced'),
        'complained', count(*) FILTER (WHERE d.state = 'complained'), 'skipped', count(*) FILTER (WHERE d.state = 'skipped'),
        'replied', count(*) FILTER (WHERE d.replied_at IS NOT NULL), 'auto_replied', count(*) FILTER (WHERE d.auto_replied_at IS NOT NULL))
    FROM status_notice_deliveries d WHERE d.workspace_id = n.workspace_id AND d.notice_id = n.id), updated_at = @now
WHERE n.workspace_id = @workspace_id AND n.id = @id
RETURNING *;

-- name: CountStatusNoticeMailsSince :one
SELECT count(*)::bigint FROM status_notice_deliveries
WHERE workspace_id = @workspace_id AND sent_at >= @since::timestamptz;

-- name: StatusSubscriptionActive :one
SELECT EXISTS (
    SELECT 1 FROM list_subscriptions WHERE workspace_id = @workspace_id AND id = @id AND state = 'subscribed'
) AS active;

-- name: FindStatusNoticeDeliveryByHeader :one
SELECT * FROM status_notice_deliveries
WHERE workspace_id = @workspace_id AND (header_message_id = @header_message_id OR relay_message_id = @header_message_id)
LIMIT 1;

-- name: FindStatusNoticeDeliveryAnyWorkspace :one
SELECT d.* FROM status_notice_deliveries d
JOIN workspaces w ON w.id = d.workspace_id
WHERE d.header_message_id = @header_message_id AND w.deleted_at IS NULL;

-- name: FindStatusNoticeDeliveryByTokens :one
SELECT d.* FROM status_notice_deliveries d
JOIN status_notices n ON n.workspace_id = d.workspace_id AND n.id = d.notice_id
JOIN status_pages p ON p.workspace_id = n.workspace_id AND p.id = n.page_id
WHERE d.workspace_id = @workspace_id AND p.inbox_id = @inbox_id
  AND (d.email_token = ANY(@tokens::text[]) OR d.relay_message_id = ANY(@ids::text[]))
ORDER BY (d.contact_id = @contact_id) DESC, d.created_at DESC
LIMIT 1;

-- name: MarkStatusDeliveryBounced :execrows
UPDATE status_notice_deliveries SET state = 'bounced', bounced_at = @now::timestamptz, next_attempt_at = NULL, error = coalesce(sqlc.narg(error), error)
WHERE workspace_id = @workspace_id AND id = @id AND state IN ('queued', 'sent');

-- name: MarkStatusDeliveryComplained :execrows
UPDATE status_notice_deliveries SET state = 'complained', complained_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND state IN ('sent', 'bounced');

-- name: MarkStatusDeliveryAutoReplied :execrows
UPDATE status_notice_deliveries SET auto_replied_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND auto_replied_at IS NULL;

-- name: MarkStatusDeliveryReplied :execrows
UPDATE status_notice_deliveries SET replied_at = @now::timestamptz, conversation_id = sqlc.narg(conversation_id)
WHERE workspace_id = @workspace_id AND id = @id AND replied_at IS NULL;

-- name: StatusNoticeIncident :one
SELECT i.* FROM status_incidents i
JOIN status_notices n ON n.workspace_id = i.workspace_id AND n.incident_id = i.id
WHERE n.workspace_id = @workspace_id AND n.id = @notice_id;

-- name: DeleteOldStatusNoticeDeliveries :execrows
DELETE FROM status_notice_deliveries WHERE (workspace_id, id) IN (
    SELECT d.workspace_id, d.id FROM status_notice_deliveries d
    WHERE d.workspace_id = @workspace_id AND d.created_at < @before::timestamptz
    LIMIT @lim);

-- name: AddStatusNoticeUsage :exec
INSERT INTO usage_counters (workspace_id, month, status_notices)
VALUES (@workspace_id, @month, @notices)
ON CONFLICT (workspace_id, month) DO UPDATE SET status_notices = usage_counters.status_notices + excluded.status_notices;
