-- +goose Up
-- An export file arrives in numbered parts of at most 8 MiB, each stored as an object of its own;
-- the parts read in order are the file. They are deleted when the migration ends.
CREATE TABLE migration_upload_parts (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    migration_id uuid NOT NULL,
    part         integer NOT NULL CHECK (part >= 0),
    size_bytes   bigint NOT NULL CHECK (size_bytes BETWEEN 1 AND 8388608),
    sha256       text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    storage_key  text NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, migration_id, part),
    FOREIGN KEY (workspace_id, migration_id) REFERENCES migrations (workspace_id, id) ON DELETE CASCADE
);

-- upload is a file migration's export file: its name, size, format, what the scan needs to read
-- it again and how far the scan read. follows_migration_id is the file migration of the same
-- mailbox and channel an IMAP migration continues from.
ALTER TABLE migrations
    ADD COLUMN upload jsonb,
    ADD COLUMN follows_migration_id uuid,
    ADD CONSTRAINT migrations_follows_fkey FOREIGN KEY (workspace_id, follows_migration_id)
        REFERENCES migrations (workspace_id, id) ON DELETE SET NULL (follows_migration_id);

ALTER TABLE migration_audit DROP CONSTRAINT migration_audit_action_check;
ALTER TABLE migration_audit ADD CONSTRAINT migration_audit_action_check CHECK (action IN ('created', 'credentials_set', 'signed_in',
    'tested', 'scanned', 'started', 'paused', 'resumed', 'cancelled', 'finished', 'credentials_deleted', 'history_removed', 'deleted',
    'probe_sent', 'delivery_verified', 'needs_attention', 'retried', 'upload_completed', 'upload_refused', 'file_deleted'));

-- +goose Down
DELETE FROM migration_audit WHERE action IN ('upload_completed', 'upload_refused', 'file_deleted');
ALTER TABLE migration_audit DROP CONSTRAINT migration_audit_action_check;
ALTER TABLE migration_audit ADD CONSTRAINT migration_audit_action_check CHECK (action IN ('created', 'credentials_set', 'signed_in',
    'tested', 'scanned', 'started', 'paused', 'resumed', 'cancelled', 'finished', 'credentials_deleted', 'history_removed', 'deleted',
    'probe_sent', 'delivery_verified', 'needs_attention', 'retried'));
ALTER TABLE migrations DROP CONSTRAINT migrations_follows_fkey, DROP COLUMN follows_migration_id, DROP COLUMN upload;
DROP TABLE migration_upload_parts;
