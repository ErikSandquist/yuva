-- +goose Up
ALTER TABLE email_channels ALTER COLUMN forward_token SET DEFAULT substr(md5(gen_random_uuid()::text), 1, 16);
UPDATE email_channels SET forward_token = DEFAULT WHERE forward_token IS NULL;
ALTER TABLE email_channels ALTER COLUMN forward_token SET NOT NULL;

-- quiet_since is the last time catch-up sync stored mail that had not also reached Yuva directly.
ALTER TABLE migrations
    ADD COLUMN synced_at       timestamptz,
    ADD COLUMN quiet_since     timestamptz,
    ADD COLUMN probe_sent_at   timestamptz,
    ADD COLUMN probe_synced_at timestamptz;

CREATE INDEX migration_audit_created_idx ON migration_audit (workspace_id, created_at);

-- +goose Down
DROP INDEX migration_audit_created_idx;
ALTER TABLE migrations DROP COLUMN probe_synced_at, DROP COLUMN probe_sent_at, DROP COLUMN quiet_since, DROP COLUMN synced_at;
ALTER TABLE email_channels ALTER COLUMN forward_token DROP NOT NULL;
ALTER TABLE email_channels ALTER COLUMN forward_token DROP DEFAULT;
