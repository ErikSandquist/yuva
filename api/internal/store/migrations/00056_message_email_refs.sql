-- +goose Up
CREATE INDEX message_emails_in_reply_to_idx ON message_emails (workspace_id, in_reply_to) WHERE in_reply_to IS NOT NULL;
CREATE INDEX message_emails_references_idx ON message_emails USING gin (references_ids);

-- +goose Down
DROP INDEX message_emails_references_idx;
DROP INDEX message_emails_in_reply_to_idx;
