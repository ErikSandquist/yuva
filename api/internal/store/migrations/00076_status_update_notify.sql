-- +goose Up
ALTER TABLE status_incident_updates ADD COLUMN notify boolean NOT NULL DEFAULT false;
UPDATE status_incident_updates SET notify = true WHERE notice_id IS NOT NULL;
UPDATE status_incident_updates u SET notify = true
FROM status_incidents i
WHERE i.workspace_id = u.workspace_id AND i.id = u.incident_id AND i.notify
  AND u.id = (SELECT o.id FROM status_incident_updates o
              WHERE o.workspace_id = i.workspace_id AND o.incident_id = i.id
              ORDER BY o.created_at, o.id LIMIT 1);
ALTER TABLE status_incident_updates ALTER COLUMN notify DROP DEFAULT;

ALTER TABLE status_notices DROP CONSTRAINT status_notices_skip_reason_check;
ALTER TABLE status_notices ADD CONSTRAINT status_notices_skip_reason_check
    CHECK (skip_reason IN ('rate', 'suspended', 'email_off', 'unpublished', 'sender_not_ready', 'no_recipients', 'nothing_earlier', 'closed_quietly'));

-- +goose Down
UPDATE status_notices SET skip_reason = NULL WHERE skip_reason = 'closed_quietly';
ALTER TABLE status_notices DROP CONSTRAINT status_notices_skip_reason_check;
ALTER TABLE status_notices ADD CONSTRAINT status_notices_skip_reason_check
    CHECK (skip_reason IN ('rate', 'suspended', 'email_off', 'unpublished', 'sender_not_ready', 'no_recipients', 'nothing_earlier'));
ALTER TABLE status_incident_updates DROP COLUMN notify;
