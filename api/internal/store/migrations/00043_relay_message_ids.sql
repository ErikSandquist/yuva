-- +goose Up
-- relay_message_id is the Message-ID a relay such as Amazon SES put on the mail in place of ours;
-- replies and reports name it instead of header_message_id.
ALTER TABLE message_emails ADD COLUMN relay_message_id text CHECK (length(relay_message_id) BETWEEN 1 AND 998);
CREATE INDEX message_emails_relay_idx ON message_emails (workspace_id, relay_message_id) WHERE relay_message_id IS NOT NULL;

ALTER TABLE broadcast_deliveries ADD COLUMN relay_message_id text CHECK (length(relay_message_id) BETWEEN 1 AND 998);
CREATE INDEX broadcast_deliveries_relay_idx ON broadcast_deliveries (workspace_id, relay_message_id) WHERE relay_message_id IS NOT NULL;

-- +goose Down
DROP INDEX broadcast_deliveries_relay_idx;
ALTER TABLE broadcast_deliveries DROP COLUMN relay_message_id;
DROP INDEX message_emails_relay_idx;
ALTER TABLE message_emails DROP COLUMN relay_message_id;
