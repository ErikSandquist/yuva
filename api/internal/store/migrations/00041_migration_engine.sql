-- +goose Up
-- meta holds what the scan read from a message's headers (addresses, subject, date, parts,
-- references); in_copy marks the items the copy took on, which its counts are made of.
ALTER TABLE migration_items
    ADD COLUMN meta    jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN in_copy boolean NOT NULL DEFAULT false;
CREATE INDEX migration_items_dedupe_idx ON migration_items (workspace_id, migration_id, dedupe_key);
CREATE INDEX migration_items_folder_idx ON migration_items (workspace_id, migration_id, folder_id);

ALTER TABLE migration_threads ADD COLUMN plan jsonb NOT NULL DEFAULT '{}';

ALTER TABLE migration_folders
    ADD COLUMN delim      text NOT NULL DEFAULT '' CHECK (length(delim) <= 4),
    ADD COLUMN uidnext    bigint NOT NULL DEFAULT 0,
    ADD COLUMN size_bytes bigint NOT NULL DEFAULT 0,
    ADD COLUMN oldest_at  timestamptz,
    ADD COLUMN newest_at  timestamptz;
CREATE UNIQUE INDEX migration_folders_name_key ON migration_folders (workspace_id, migration_id, name);

ALTER TABLE migration_audit DROP CONSTRAINT migration_audit_action_check;
ALTER TABLE migration_audit ADD CONSTRAINT migration_audit_action_check CHECK (action IN ('created', 'credentials_set', 'signed_in',
    'tested', 'scanned', 'started', 'paused', 'resumed', 'cancelled', 'finished', 'credentials_deleted', 'history_removed', 'deleted',
    'probe_sent', 'delivery_verified', 'needs_attention', 'retried'));

-- An import looks a message up by its Message-ID on the channel in either direction.
CREATE INDEX message_emails_channel_header_idx ON message_emails (workspace_id, channel_id, header_message_id);

-- +goose Down
DELETE FROM migration_audit WHERE action = 'retried';
ALTER TABLE migration_audit DROP CONSTRAINT migration_audit_action_check;
ALTER TABLE migration_audit ADD CONSTRAINT migration_audit_action_check CHECK (action IN ('created', 'credentials_set', 'signed_in',
    'tested', 'scanned', 'started', 'paused', 'resumed', 'cancelled', 'finished', 'credentials_deleted', 'history_removed', 'deleted',
    'probe_sent', 'delivery_verified', 'needs_attention'));
DROP INDEX message_emails_channel_header_idx;
DROP INDEX migration_folders_name_key;
ALTER TABLE migration_folders DROP COLUMN newest_at, DROP COLUMN oldest_at, DROP COLUMN size_bytes, DROP COLUMN uidnext, DROP COLUMN delim;
ALTER TABLE migration_threads DROP COLUMN plan;
DROP INDEX migration_items_folder_idx;
DROP INDEX migration_items_dedupe_idx;
ALTER TABLE migration_items DROP COLUMN in_copy, DROP COLUMN meta;
