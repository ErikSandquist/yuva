-- +goose Up
-- A help-desk sign-in between its start and the help desk's redirect back: the team's OAuth
-- client and the PKCE verifier, sealed like credentials. One per migration; the state's hash
-- takes it once, within ten minutes.
CREATE TABLE migration_sign_ins (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    migration_id uuid NOT NULL,
    state_hash   text NOT NULL CHECK (state_hash ~ '^[0-9a-f]{64}$'),
    sealed       bytea NOT NULL,
    host         text NOT NULL CHECK (length(host) BETWEEN 1 AND 253),
    member_id    uuid,
    api_key_id   uuid,
    expires_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, migration_id),
    FOREIGN KEY (workspace_id, migration_id) REFERENCES migrations (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, member_id) REFERENCES members (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, api_key_id) REFERENCES api_keys (workspace_id, id) ON DELETE CASCADE
);

-- source_state is a help desk's: where the scan's and sync's ticket listings stand, and how the
-- last sign-in ended.
ALTER TABLE migrations ADD COLUMN source_state jsonb NOT NULL DEFAULT '{}';

-- A help desk's brand or mailbox: its id there and its support address.
ALTER TABLE migration_folders
    ADD COLUMN source_ref text CHECK (length(source_ref) BETWEEN 1 AND 200),
    ADD COLUMN address    text CHECK (length(address) <= 320);

ALTER TABLE migration_audit DROP CONSTRAINT migration_audit_action_check;
ALTER TABLE migration_audit ADD CONSTRAINT migration_audit_action_check CHECK (action IN ('created', 'credentials_set', 'signed_in',
    'tested', 'scanned', 'started', 'paused', 'resumed', 'cancelled', 'finished', 'credentials_deleted', 'history_removed', 'deleted',
    'probe_sent', 'delivery_verified', 'needs_attention', 'retried', 'upload_completed', 'upload_refused', 'file_deleted',
    'sign_in_started', 'sign_in_failed'));

-- +goose Down
DELETE FROM migration_audit WHERE action IN ('sign_in_started', 'sign_in_failed');
ALTER TABLE migration_audit DROP CONSTRAINT migration_audit_action_check;
ALTER TABLE migration_audit ADD CONSTRAINT migration_audit_action_check CHECK (action IN ('created', 'credentials_set', 'signed_in',
    'tested', 'scanned', 'started', 'paused', 'resumed', 'cancelled', 'finished', 'credentials_deleted', 'history_removed', 'deleted',
    'probe_sent', 'delivery_verified', 'needs_attention', 'retried', 'upload_completed', 'upload_refused', 'file_deleted'));
ALTER TABLE migration_folders DROP COLUMN address, DROP COLUMN source_ref;
ALTER TABLE migrations DROP COLUMN source_state;
DROP TABLE migration_sign_ins;
