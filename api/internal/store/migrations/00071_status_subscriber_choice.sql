-- +goose Up
ALTER TABLE list_subscriptions ADD COLUMN chose_components boolean NOT NULL DEFAULT false;
UPDATE list_subscriptions s SET chose_components = true
WHERE EXISTS (SELECT 1 FROM status_subscriber_components c WHERE c.workspace_id = s.workspace_id AND c.subscription_id = s.id);

ALTER TABLE status_notices DROP CONSTRAINT status_notices_skip_reason_check;
ALTER TABLE status_notices ADD CONSTRAINT status_notices_skip_reason_check
    CHECK (skip_reason IN ('rate', 'suspended', 'email_off', 'unpublished', 'sender_not_ready', 'no_recipients', 'nothing_earlier'));

-- +goose Down
UPDATE status_notices SET skip_reason = 'no_recipients' WHERE skip_reason = 'nothing_earlier';
ALTER TABLE status_notices DROP CONSTRAINT status_notices_skip_reason_check;
ALTER TABLE status_notices ADD CONSTRAINT status_notices_skip_reason_check
    CHECK (skip_reason IN ('rate', 'suspended', 'email_off', 'unpublished', 'sender_not_ready', 'no_recipients'));
ALTER TABLE list_subscriptions DROP COLUMN chose_components;
