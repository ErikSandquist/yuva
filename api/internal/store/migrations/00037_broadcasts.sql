-- +goose Up
CREATE TABLE broadcasts (
    id               uuid PRIMARY KEY,
    workspace_id     uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    inbox_id         uuid NOT NULL,
    channel_id       uuid,
    name             text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    subject          text NOT NULL DEFAULT '' CHECK (length(subject) <= 300),
    preheader        text NOT NULL DEFAULT '' CHECK (length(preheader) <= 300),
    design_id        uuid,
    design_version   integer,
    document         jsonb,
    audience         jsonb NOT NULL DEFAULT '{"list_ids": [], "conditions": [], "locales": []}',
    click_tracking   boolean NOT NULL DEFAULT false,
    state            text NOT NULL DEFAULT 'draft'
        CHECK (state IN ('draft', 'scheduled', 'preparing', 'sending', 'paused', 'sent', 'cancelled', 'failed')),
    pause_reason     text CHECK (pause_reason IN ('member', 'bounce_rate', 'complaint_rate', 'sender_error', 'workspace_suspended', 'operator')),
    state_detail     text CHECK (length(state_detail) <= 1000),
    scheduled_at     timestamptz,
    waiting_until    timestamptz,
    started_at       timestamptz,
    resumed_at       timestamptz,
    finished_at      timestamptz,
    last_test_at     timestamptz,
    snapshot_after   uuid,
    snapshot_done    boolean NOT NULL DEFAULT false,
    sender_failing_since timestamptz,
    counts           jsonb NOT NULL DEFAULT '{}',
    created_by       uuid,
    scheduled_by     uuid,
    scheduled_by_key uuid,
    created_at       timestamptz NOT NULL,
    updated_at       timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, channel_id) REFERENCES channels (workspace_id, id) ON DELETE SET NULL (channel_id),
    FOREIGN KEY (workspace_id, design_id) REFERENCES email_designs (workspace_id, id) ON DELETE SET NULL (design_id),
    FOREIGN KEY (workspace_id, created_by) REFERENCES members (workspace_id, id) ON DELETE SET NULL (created_by),
    FOREIGN KEY (workspace_id, scheduled_by) REFERENCES members (workspace_id, id) ON DELETE SET NULL (scheduled_by)
);
CREATE INDEX broadcasts_list_idx ON broadcasts (workspace_id, created_at DESC, id DESC);
CREATE INDEX broadcasts_active_idx ON broadcasts (workspace_id, state) WHERE state IN ('scheduled', 'preparing', 'sending', 'paused');
CREATE INDEX broadcasts_channel_idx ON broadcasts (workspace_id, channel_id, started_at) WHERE state = 'sending';

-- header_message_id is looked up without a workspace by /ingress/ses, as message_emails is
-- (architecture.md, Hosting for others later).
CREATE TABLE broadcast_deliveries (
    id                uuid PRIMARY KEY,
    workspace_id      uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    broadcast_id      uuid NOT NULL,
    contact_id        uuid NOT NULL,
    list_id           uuid,
    email             text NOT NULL,
    email_token       text NOT NULL CHECK (email_token ~ '^[a-z2-7]{24}$'),
    header_message_id text NOT NULL,
    state             text NOT NULL DEFAULT 'queued'
        CHECK (state IN ('queued', 'sent', 'failed', 'bounced', 'complained', 'skipped')),
    skip_reason       text CHECK (skip_reason IN ('suppressed', 'opted_out', 'unsubscribed', 'cancelled')),
    attempts          integer NOT NULL DEFAULT 0,
    next_attempt_at   timestamptz,
    error             text CHECK (length(error) <= 500),
    sent_at           timestamptz,
    bounced_at        timestamptz,
    complained_at     timestamptz,
    unsubscribed_at   timestamptz,
    replied_at        timestamptz,
    auto_replied_at   timestamptz,
    clicked_at        timestamptz,
    conversation_id   uuid,
    created_at        timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, broadcast_id, contact_id),
    UNIQUE (workspace_id, email_token),
    FOREIGN KEY (workspace_id, broadcast_id) REFERENCES broadcasts (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, contact_id) REFERENCES contacts (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, list_id) REFERENCES mailing_lists (workspace_id, id) ON DELETE SET NULL (list_id),
    FOREIGN KEY (workspace_id, conversation_id) REFERENCES conversations (workspace_id, id) ON DELETE SET NULL (conversation_id)
);
CREATE INDEX broadcast_deliveries_queue_idx ON broadcast_deliveries (workspace_id, broadcast_id, next_attempt_at NULLS FIRST, id)
    WHERE state = 'queued';
CREATE INDEX broadcast_deliveries_list_idx ON broadcast_deliveries (workspace_id, broadcast_id, id);
CREATE INDEX broadcast_deliveries_contact_idx ON broadcast_deliveries (workspace_id, contact_id);
CREATE UNIQUE INDEX broadcast_deliveries_header_key ON broadcast_deliveries (header_message_id);
CREATE INDEX broadcast_deliveries_age_idx ON broadcast_deliveries (workspace_id, created_at);

CREATE TABLE broadcast_links (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    broadcast_id uuid NOT NULL,
    position     integer NOT NULL CHECK (position BETWEEN 0 AND 499),
    url          text NOT NULL CHECK (length(url) <= 2048),
    clicks       bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_id, broadcast_id, position),
    FOREIGN KEY (workspace_id, broadcast_id) REFERENCES broadcasts (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE broadcast_clicks (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    delivery_id  uuid NOT NULL,
    position     integer NOT NULL,
    first_at     timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, delivery_id, position),
    FOREIGN KEY (workspace_id, delivery_id) REFERENCES broadcast_deliveries (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE broadcast_sending_days (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    day          date NOT NULL,
    sent         integer NOT NULL DEFAULT 0,
    bounced      integer NOT NULL DEFAULT 0,
    complained   integer NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_id, day)
);

ALTER TABLE conversations
    ADD COLUMN broadcast_id uuid,
    ADD CONSTRAINT conversations_broadcast_fkey FOREIGN KEY (workspace_id, broadcast_id)
        REFERENCES broadcasts (workspace_id, id) ON DELETE SET NULL (broadcast_id);
CREATE INDEX conversations_broadcast_idx ON conversations (workspace_id, broadcast_id, created_at) WHERE broadcast_id IS NOT NULL;

ALTER TABLE operator_notices
    DROP CONSTRAINT operator_notices_kind_check,
    ADD CONSTRAINT operator_notices_kind_check
        CHECK (kind IN ('signup', 'workspace', 'broadcast_approval', 'broadcast_suspended', 'broadcast_paused')),
    ADD COLUMN detail text;

-- +goose Down
DELETE FROM operator_notices WHERE kind NOT IN ('signup', 'workspace');
ALTER TABLE operator_notices
    DROP COLUMN detail,
    DROP CONSTRAINT operator_notices_kind_check,
    ADD CONSTRAINT operator_notices_kind_check CHECK (kind IN ('signup', 'workspace'));
DROP INDEX conversations_broadcast_idx;
ALTER TABLE conversations DROP CONSTRAINT conversations_broadcast_fkey, DROP COLUMN broadcast_id;
DROP TABLE broadcast_sending_days;
DROP TABLE broadcast_clicks;
DROP TABLE broadcast_links;
DROP TABLE broadcast_deliveries;
DROP TABLE broadcasts;
