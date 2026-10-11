-- +goose Up
CREATE TABLE mailing_lists (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    inbox_id      uuid NOT NULL,
    channel_id    uuid,
    name          text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    description   text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    double_opt_in boolean NOT NULL DEFAULT true,
    public        boolean NOT NULL DEFAULT true,
    commercial    boolean NOT NULL DEFAULT true,
    welcome       boolean NOT NULL DEFAULT true,
    archived_at   timestamptz,
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, inbox_id, name),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, channel_id) REFERENCES channels (workspace_id, id) ON DELETE SET NULL (channel_id)
);
CREATE INDEX mailing_lists_list_idx ON mailing_lists (workspace_id, created_at DESC, id DESC);

CREATE TABLE list_subscriptions (
    id                       uuid PRIMARY KEY,
    workspace_id             uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    list_id                  uuid NOT NULL,
    contact_id               uuid NOT NULL,
    email                    text NOT NULL CHECK (email = lower(email) AND length(email) BETWEEN 3 AND 320),
    state                    text NOT NULL CHECK (state IN ('pending', 'subscribed', 'unsubscribed')),
    source                   text NOT NULL CHECK (source IN ('form', 'identity', 'api', 'member', 'import', 'preferences')),
    consent_at               timestamptz,
    confirmed_at             timestamptz,
    confirmation_sent_at     timestamptz,
    confirmations_sent       integer NOT NULL DEFAULT 0,
    unsubscribed_at          timestamptz,
    unsubscribe_reason       text CHECK (unsubscribe_reason IN ('link', 'one_click', 'preferences', 'client', 'member', 'api', 'complaint', 'erasure')),
    unsubscribe_broadcast_id uuid,
    created_at               timestamptz NOT NULL,
    updated_at               timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, list_id, email),
    CHECK (state <> 'subscribed' OR consent_at IS NOT NULL),
    CHECK ((state = 'unsubscribed') = (unsubscribed_at IS NOT NULL)),
    FOREIGN KEY (workspace_id, list_id) REFERENCES mailing_lists (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, contact_id) REFERENCES contacts (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX list_subscriptions_list_idx    ON list_subscriptions (workspace_id, list_id, state, created_at DESC, id DESC);
CREATE INDEX list_subscriptions_recent_idx  ON list_subscriptions (workspace_id, list_id, created_at DESC, id DESC);
CREATE INDEX list_subscriptions_contact_idx ON list_subscriptions (workspace_id, contact_id);
CREATE INDEX list_subscriptions_email_idx   ON list_subscriptions (workspace_id, email);
CREATE INDEX list_subscriptions_pending_idx ON list_subscriptions (workspace_id, created_at) WHERE state = 'pending';
CREATE INDEX list_subscriptions_unsubscribed_idx ON list_subscriptions (workspace_id, unsubscribed_at) WHERE state = 'unsubscribed';

-- Append-only proof of consent: no query updates a row; rows go with their subscription.
CREATE TABLE consent_events (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    subscription_id uuid NOT NULL,
    kind            text NOT NULL CHECK (kind IN ('requested', 'confirmation_sent', 'confirmed', 'subscribed', 'unsubscribed', 'complained')),
    source          text NOT NULL CHECK (length(source) BETWEEN 1 AND 40),
    ip              text CHECK (length(ip) <= 64),
    user_agent      text CHECK (length(user_agent) <= 300),
    page_url        text CHECK (length(page_url) <= 2048),
    consent_text    text CHECK (length(consent_text) <= 2000),
    member_id       uuid,
    api_key_id      uuid,
    import_id       uuid,
    broadcast_id    uuid,
    detail          jsonb NOT NULL DEFAULT '{}',
    created_at      timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, subscription_id) REFERENCES list_subscriptions (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX consent_events_subscription_idx ON consent_events (workspace_id, subscription_id, created_at, id);

-- Objections kept after the address itself is gone: an HMAC of the address under a key derived
-- from the master key and the workspace, nothing else.
CREATE TABLE email_optouts (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    email_hash   bytea NOT NULL,
    reason       text NOT NULL CHECK (reason IN ('unsubscribed_all', 'complaint', 'erasure_after_optout')),
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, email_hash)
);

-- +goose Down
DROP TABLE email_optouts;
DROP TABLE consent_events;
DROP TABLE list_subscriptions;
DROP TABLE mailing_lists;
