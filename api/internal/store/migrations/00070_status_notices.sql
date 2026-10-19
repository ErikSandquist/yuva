-- +goose Up
ALTER TABLE status_incidents ADD COLUMN notify boolean NOT NULL DEFAULT true;

CREATE TABLE status_notices (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    page_id       uuid NOT NULL,
    incident_id   uuid NOT NULL,
    state         text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'sending', 'sent', 'skipped', 'cancelled')),
    final         boolean NOT NULL DEFAULT false,
    due_at        timestamptz NOT NULL,
    skip_reason   text CHECK (skip_reason IN ('rate', 'suspended', 'email_off', 'unpublished', 'sender_not_ready', 'no_recipients')),
    counts        jsonb NOT NULL DEFAULT '{}',
    snapshot_done boolean NOT NULL DEFAULT false,
    lease_until   timestamptz,
    started_at    timestamptz,
    finished_at   timestamptz,
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, page_id) REFERENCES status_pages (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, incident_id) REFERENCES status_incidents (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX status_notices_page_idx ON status_notices (workspace_id, page_id, created_at DESC, id DESC);
CREATE INDEX status_notices_incident_idx ON status_notices (workspace_id, incident_id, created_at DESC);
CREATE INDEX status_notices_due_idx ON status_notices (due_at) WHERE state = 'pending';
CREATE INDEX status_notices_lease_idx ON status_notices (lease_until) WHERE state = 'sending';
CREATE UNIQUE INDEX status_notices_pending_key ON status_notices (workspace_id, incident_id) WHERE state = 'pending';

ALTER TABLE status_incident_updates ADD COLUMN notice_id uuid;
ALTER TABLE status_incident_updates ADD CONSTRAINT status_incident_updates_notice_fkey
    FOREIGN KEY (workspace_id, notice_id) REFERENCES status_notices (workspace_id, id) ON DELETE SET NULL (notice_id);
CREATE INDEX status_incident_updates_notice_idx ON status_incident_updates (workspace_id, notice_id) WHERE notice_id IS NOT NULL;

CREATE TABLE status_notice_deliveries (
    id                  uuid PRIMARY KEY,
    workspace_id        uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    notice_id           uuid NOT NULL,
    subscription_id     uuid NOT NULL,
    contact_id          uuid NOT NULL,
    email               text NOT NULL,
    email_token         text NOT NULL CHECK (email_token ~ '^[a-z2-7]{24}$'),
    header_message_id   text NOT NULL,
    previous_message_id text,
    relay_message_id    text CHECK (length(relay_message_id) BETWEEN 1 AND 998),
    state               text NOT NULL DEFAULT 'queued'
        CHECK (state IN ('queued', 'sent', 'failed', 'bounced', 'complained', 'skipped')),
    skip_reason         text CHECK (skip_reason IN ('unsubscribed', 'suppressed', 'limit', 'stale')),
    attempts            integer NOT NULL DEFAULT 0,
    next_attempt_at     timestamptz,
    error               text CHECK (length(error) <= 500),
    sent_at             timestamptz,
    bounced_at          timestamptz,
    complained_at       timestamptz,
    replied_at          timestamptz,
    auto_replied_at     timestamptz,
    conversation_id     uuid,
    created_at          timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, notice_id, subscription_id),
    UNIQUE (workspace_id, email_token),
    FOREIGN KEY (workspace_id, notice_id) REFERENCES status_notices (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, contact_id) REFERENCES contacts (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, conversation_id) REFERENCES conversations (workspace_id, id) ON DELETE SET NULL (conversation_id)
);
CREATE UNIQUE INDEX status_notice_deliveries_header_key ON status_notice_deliveries (header_message_id);
CREATE INDEX status_notice_deliveries_queue_idx ON status_notice_deliveries (workspace_id, notice_id, next_attempt_at NULLS FIRST, id)
    WHERE state = 'queued';
CREATE INDEX status_notice_deliveries_relay_idx ON status_notice_deliveries (workspace_id, relay_message_id) WHERE relay_message_id IS NOT NULL;
CREATE INDEX status_notice_deliveries_subscription_idx ON status_notice_deliveries (workspace_id, subscription_id, created_at DESC);
CREATE INDEX status_notice_deliveries_contact_idx ON status_notice_deliveries (workspace_id, contact_id);
CREATE INDEX status_notice_deliveries_day_idx ON status_notice_deliveries (workspace_id, sent_at) WHERE sent_at IS NOT NULL;
CREATE INDEX status_notice_deliveries_age_idx ON status_notice_deliveries (workspace_id, created_at);

ALTER TABLE usage_counters ADD COLUMN status_notices bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE usage_counters DROP COLUMN status_notices;
DROP TABLE status_notice_deliveries;
DROP INDEX status_incident_updates_notice_idx;
ALTER TABLE status_incident_updates DROP CONSTRAINT status_incident_updates_notice_fkey;
ALTER TABLE status_incident_updates DROP COLUMN notice_id;
DROP TABLE status_notices;
ALTER TABLE status_incidents DROP COLUMN notify;
