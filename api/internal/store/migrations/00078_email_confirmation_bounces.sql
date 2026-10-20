-- +goose Up
ALTER TABLE email_confirmations
    ADD COLUMN header_message_id text,
    ADD COLUMN relay_message_id text,
    ADD COLUMN bounced_at timestamptz,
    ADD COLUMN complained_at timestamptz,
    ADD COLUMN confirmed_at timestamptz;
CREATE UNIQUE INDEX email_confirmations_header_idx ON email_confirmations (header_message_id) WHERE header_message_id IS NOT NULL;
CREATE INDEX email_confirmations_relay_idx ON email_confirmations (relay_message_id) WHERE relay_message_id IS NOT NULL;
CREATE INDEX email_confirmations_latest_idx ON email_confirmations (workspace_id, contact_id, created_at DESC);

ALTER TABLE email_suppressions
    DROP CONSTRAINT email_suppressions_scope_check,
    ADD CONSTRAINT email_suppressions_scope_check CHECK (scope IN ('all', 'broadcasts', 'confirmations'));

-- +goose Down
DELETE FROM email_suppressions WHERE scope = 'confirmations';
ALTER TABLE email_suppressions
    DROP CONSTRAINT email_suppressions_scope_check,
    ADD CONSTRAINT email_suppressions_scope_check CHECK (scope IN ('all', 'broadcasts'));

DROP INDEX email_confirmations_latest_idx;
DROP INDEX email_confirmations_relay_idx;
DROP INDEX email_confirmations_header_idx;
ALTER TABLE email_confirmations
    DROP COLUMN confirmed_at,
    DROP COLUMN complained_at,
    DROP COLUMN bounced_at,
    DROP COLUMN relay_message_id,
    DROP COLUMN header_message_id;
