-- +goose Up
CREATE TABLE migrations (
    id                      uuid PRIMARY KEY,
    workspace_id            uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    source                  text NOT NULL CHECK (source IN ('imap', 'file', 'zendesk', 'helpscout')),
    preset                  text NOT NULL DEFAULT 'imap' CHECK (preset ~ '^[a-z0-9_]{1,40}$'),
    source_address          text NOT NULL DEFAULT '' CHECK (length(source_address) <= 320),
    inbox_id                uuid,
    channel_id              uuid,
    state                   text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'connected', 'scanning', 'ready',
                              'copying', 'paused', 'needs_attention', 'syncing', 'completed', 'cancelled', 'removing', 'removed')),
    state_reason            text NOT NULL DEFAULT '' CHECK (length(state_reason) <= 500),
    waiting_until           timestamptz,
    options                 jsonb NOT NULL DEFAULT '{}',
    estimate                jsonb,
    counts                  jsonb NOT NULL DEFAULT '{}',
    throttle                jsonb NOT NULL DEFAULT '{}',
    checkpoint              jsonb NOT NULL DEFAULT '{}',
    live_from               timestamptz,
    verified_at             timestamptz,
    verified_route          text CHECK (verified_route IN ('forwarding', 'direct')),
    forwarding_confirmation jsonb,
    lease_until             timestamptz,
    lease_job_id            bigint,
    created_by_member_id    uuid,
    created_by_api_key_id   uuid,
    created_via             text CHECK (length(created_via) BETWEEN 1 AND 200),
    created_at              timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL,
    started_at              timestamptz,
    finished_at             timestamptz,
    credentials_deleted_at  timestamptz,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE SET NULL (inbox_id),
    FOREIGN KEY (workspace_id, channel_id) REFERENCES channels (workspace_id, id) ON DELETE SET NULL (channel_id),
    FOREIGN KEY (workspace_id, created_by_member_id) REFERENCES members (workspace_id, id) ON DELETE SET NULL (created_by_member_id),
    FOREIGN KEY (workspace_id, created_by_api_key_id) REFERENCES api_keys (workspace_id, id) ON DELETE SET NULL (created_by_api_key_id)
);
CREATE INDEX migrations_list_idx ON migrations (workspace_id, created_at DESC, id DESC);
-- The watchdog looks across workspaces (see "Hosting for others later" in docs/architecture.md).
CREATE INDEX migrations_lease_idx ON migrations (lease_until) WHERE state IN ('scanning', 'copying', 'syncing', 'removing');

-- The secret (user name and password, or tokens) is sealed with context "migration:" + workspace +
-- migration; the server it is sent to and a pinned certificate are not secret.
CREATE TABLE migration_credentials (
    workspace_id       uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    migration_id       uuid NOT NULL,
    kind               text NOT NULL CHECK (kind IN ('password', 'oauth')),
    sealed             bytea NOT NULL,
    host               text NOT NULL CHECK (length(host) BETWEEN 1 AND 253),
    port               integer NOT NULL CHECK (port BETWEEN 1 AND 65535),
    tls                text NOT NULL CHECK (tls IN ('tls', 'starttls')),
    certificate_sha256 text CHECK (certificate_sha256 ~ '^[0-9a-f]{64}$'),
    scopes             text[] NOT NULL DEFAULT '{}',
    expires_at         timestamptz,
    created_at         timestamptz NOT NULL,
    updated_at         timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, migration_id),
    FOREIGN KEY (workspace_id, migration_id) REFERENCES migrations (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE migration_folders (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    migration_id uuid NOT NULL,
    id           integer NOT NULL,
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 1000),
    kind         text NOT NULL CHECK (kind IN ('inbox', 'sent', 'archive', 'all', 'user', 'label', 'spam', 'trash', 'drafts', 'other')),
    selected     boolean NOT NULL,
    label_action text NOT NULL DEFAULT 'none' CHECK (label_action IN ('none', 'create', 'existing')),
    label_id     uuid,
    uidvalidity  bigint,
    scanned_uid  bigint NOT NULL DEFAULT 0,
    seen_uid     bigint NOT NULL DEFAULT 0,
    cursor       text CHECK (length(cursor) <= 4000),
    total        integer NOT NULL DEFAULT 0,
    imported     integer NOT NULL DEFAULT 0,
    duplicate    integer NOT NULL DEFAULT 0,
    skipped      integer NOT NULL DEFAULT 0,
    failed       integer NOT NULL DEFAULT 0,
    bytes        bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_id, migration_id, id),
    FOREIGN KEY (workspace_id, migration_id) REFERENCES migrations (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, label_id) REFERENCES labels (workspace_id, id) ON DELETE SET NULL (label_id)
);

CREATE TABLE migration_threads (
    workspace_id    uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    migration_id    uuid NOT NULL,
    thread_key      text NOT NULL CHECK (length(thread_key) BETWEEN 1 AND 1000),
    conversation_id uuid,
    last_at         timestamptz NOT NULL,
    items           integer NOT NULL DEFAULT 0,
    done            integer NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_id, migration_id, thread_key),
    FOREIGN KEY (workspace_id, migration_id) REFERENCES migrations (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, conversation_id) REFERENCES conversations (workspace_id, id) ON DELETE SET NULL (conversation_id)
);
CREATE INDEX migration_threads_order_idx ON migration_threads (workspace_id, migration_id, last_at DESC) WHERE done < items;

CREATE TABLE migration_items (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    migration_id uuid NOT NULL,
    source_id    text NOT NULL CHECK (length(source_id) BETWEEN 1 AND 500),
    folder_id    integer,
    thread_key   text NOT NULL,
    dedupe_key   text NOT NULL CHECK (length(dedupe_key) BETWEEN 1 AND 998),
    -- Message-ID, In-Reply-To and References during the scan; emptied after threading.
    refs         text[] NOT NULL DEFAULT '{}',
    received_at  timestamptz NOT NULL,
    size_bytes   bigint NOT NULL CHECK (size_bytes >= 0),
    labels       text[] NOT NULL DEFAULT '{}',
    state        text NOT NULL DEFAULT 'planned' CHECK (state IN ('planned', 'imported', 'duplicate', 'skipped', 'failed')),
    reason       text NOT NULL DEFAULT '' CHECK (length(reason) <= 64),
    attempts     smallint NOT NULL DEFAULT 0,
    message_id   uuid,
    PRIMARY KEY (workspace_id, migration_id, source_id),
    FOREIGN KEY (workspace_id, migration_id) REFERENCES migrations (workspace_id, id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX migration_items_dedupe_key ON migration_items (workspace_id, migration_id, dedupe_key)
    WHERE state IN ('planned', 'imported');
CREATE INDEX migration_items_thread_idx ON migration_items (workspace_id, migration_id, thread_key, received_at);
CREATE INDEX migration_items_state_idx ON migration_items (workspace_id, migration_id, state);

CREATE TABLE migration_errors (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    migration_id uuid NOT NULL,
    code         text NOT NULL CHECK (length(code) <= 64),
    detail       text NOT NULL DEFAULT '' CHECK (length(detail) <= 2000),
    folder       text NOT NULL DEFAULT '',
    item_date    timestamptz,
    item_subject text NOT NULL DEFAULT '' CHECK (length(item_subject) <= 500),
    created_at   timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, migration_id) REFERENCES migrations (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX migration_errors_idx ON migration_errors (workspace_id, migration_id, id DESC);

-- Kept after the migration row is deleted, so no foreign key to migrations.
CREATE TABLE migration_audit (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    workspace_id     uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    migration_id     uuid NOT NULL,
    action           text NOT NULL CHECK (action IN ('created', 'credentials_set', 'signed_in', 'tested', 'scanned',
                       'started', 'paused', 'resumed', 'cancelled', 'finished', 'credentials_deleted', 'history_removed',
                       'deleted', 'probe_sent', 'delivery_verified', 'needs_attention')),
    actor_member_id  uuid,
    actor_api_key_id uuid,
    via              text,
    detail           jsonb NOT NULL DEFAULT '{}',
    created_at       timestamptz NOT NULL
);
CREATE INDEX migration_audit_idx ON migration_audit (workspace_id, migration_id, id);

ALTER TABLE conversations
    ADD COLUMN migration_id uuid,
    ADD COLUMN imported_at  timestamptz,
    ADD CONSTRAINT conversations_migration_fkey FOREIGN KEY (workspace_id, migration_id)
        REFERENCES migrations (workspace_id, id) ON DELETE SET NULL (migration_id);
CREATE INDEX conversations_migration_idx ON conversations (workspace_id, migration_id) WHERE migration_id IS NOT NULL;

ALTER TABLE messages
    ADD COLUMN imported_at     timestamptz,
    ADD COLUMN imported_author jsonb,
    ADD CONSTRAINT messages_imported_author_check CHECK (imported_author IS NULL OR (imported_at IS NOT NULL AND author_member_id IS NULL));

ALTER TABLE message_emails ADD COLUMN imported_at timestamptz;
DROP INDEX message_emails_outbound_idx;
CREATE INDEX message_emails_outbound_idx ON message_emails (header_message_id) WHERE direction = 'out' AND imported_at IS NULL;
DROP INDEX message_emails_raw_idx;
CREATE INDEX message_emails_raw_idx ON message_emails (workspace_id, (coalesce(imported_at, created_at))) WHERE raw_key IS NOT NULL;

ALTER TABLE contacts
    ADD COLUMN migration_id uuid,
    ADD CONSTRAINT contacts_migration_fkey FOREIGN KEY (workspace_id, migration_id)
        REFERENCES migrations (workspace_id, id) ON DELETE SET NULL (migration_id);
CREATE INDEX contacts_migration_idx ON contacts (workspace_id, migration_id) WHERE migration_id IS NOT NULL;

ALTER TABLE email_channels ADD COLUMN forward_token text CHECK (forward_token ~ '^[a-z0-9]{16}$');
CREATE UNIQUE INDEX email_channels_forward_token_key ON email_channels (forward_token);

ALTER TABLE usage_counters
    ADD COLUMN imported_conversations bigint NOT NULL DEFAULT 0,
    ADD COLUMN imported_messages      bigint NOT NULL DEFAULT 0,
    ADD COLUMN imported_bytes         bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE usage_counters DROP COLUMN imported_bytes, DROP COLUMN imported_messages, DROP COLUMN imported_conversations;
DROP INDEX email_channels_forward_token_key;
ALTER TABLE email_channels DROP COLUMN forward_token;
DROP INDEX contacts_migration_idx;
ALTER TABLE contacts DROP CONSTRAINT contacts_migration_fkey, DROP COLUMN migration_id;
DROP INDEX message_emails_raw_idx;
CREATE INDEX message_emails_raw_idx ON message_emails (workspace_id, created_at) WHERE raw_key IS NOT NULL;
DROP INDEX message_emails_outbound_idx;
CREATE INDEX message_emails_outbound_idx ON message_emails (header_message_id) WHERE direction = 'out';
ALTER TABLE message_emails DROP COLUMN imported_at;
ALTER TABLE messages DROP CONSTRAINT messages_imported_author_check, DROP COLUMN imported_author, DROP COLUMN imported_at;
DROP INDEX conversations_migration_idx;
ALTER TABLE conversations DROP CONSTRAINT conversations_migration_fkey, DROP COLUMN imported_at, DROP COLUMN migration_id;
DROP TABLE migration_audit;
DROP TABLE migration_errors;
DROP TABLE migration_items;
DROP TABLE migration_threads;
DROP TABLE migration_folders;
DROP TABLE migration_credentials;
DROP TABLE migrations;
