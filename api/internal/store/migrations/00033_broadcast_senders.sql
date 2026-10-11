-- +goose Up
ALTER TABLE inboxes
    ADD COLUMN sender_legal_name     text CHECK (length(sender_legal_name) BETWEEN 1 AND 200),
    ADD COLUMN sender_postal_address text CHECK (length(sender_postal_address) BETWEEN 1 AND 500),
    ADD COLUMN sender_registration   text CHECK (length(sender_registration) BETWEEN 1 AND 200),
    ADD COLUMN sender_contact        text CHECK (length(sender_contact) BETWEEN 1 AND 320);

ALTER TABLE email_channels
    ADD COLUMN broadcast_smtp_host     text CHECK (length(broadcast_smtp_host) <= 253),
    ADD COLUMN broadcast_smtp_port     integer CHECK (broadcast_smtp_port BETWEEN 1 AND 65535),
    ADD COLUMN broadcast_smtp_username text CHECK (length(broadcast_smtp_username) <= 320),
    ADD COLUMN broadcast_smtp_password bytea,
    ADD COLUMN broadcast_smtp_tls      text CHECK (broadcast_smtp_tls IN ('starttls', 'tls', 'none')),
    ADD COLUMN broadcast_rate          integer NOT NULL DEFAULT 5 CHECK (broadcast_rate BETWEEN 1 AND 100),
    ADD COLUMN dkim_selector           text CHECK (dkim_selector ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    ADD COLUMN dkim_private_key        bytea,
    ADD COLUMN dkim_enabled            boolean NOT NULL DEFAULT false;

ALTER TABLE email_suppressions
    DROP CONSTRAINT email_suppressions_reason_check,
    ADD CONSTRAINT email_suppressions_reason_check CHECK (reason IN ('bounce', 'complaint', 'manual')),
    ADD COLUMN scope text NOT NULL DEFAULT 'all' CHECK (scope IN ('all', 'broadcasts'));

ALTER TABLE workspaces
    ADD COLUMN broadcasts_status        text NOT NULL DEFAULT 'allowed'
        CHECK (broadcasts_status IN ('allowed', 'approval_required', 'approval_requested', 'suspended')),
    ADD COLUMN broadcasts_status_reason text CHECK (length(broadcasts_status_reason) <= 500),
    ADD COLUMN broadcasts_status_at     timestamptz,
    ADD COLUMN broadcast_daily_limit    integer CHECK (broadcast_daily_limit >= 0),
    ADD COLUMN broadcasts_by_keys       boolean NOT NULL DEFAULT false;

ALTER TABLE usage_counters ADD COLUMN broadcast_recipients bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE usage_counters DROP COLUMN broadcast_recipients;
ALTER TABLE workspaces
    DROP COLUMN broadcasts_by_keys,
    DROP COLUMN broadcast_daily_limit,
    DROP COLUMN broadcasts_status_at,
    DROP COLUMN broadcasts_status_reason,
    DROP COLUMN broadcasts_status;
DELETE FROM email_suppressions WHERE scope <> 'all' OR reason = 'manual';
ALTER TABLE email_suppressions
    DROP COLUMN scope,
    DROP CONSTRAINT email_suppressions_reason_check,
    ADD CONSTRAINT email_suppressions_reason_check CHECK (reason IN ('bounce', 'complaint'));
ALTER TABLE email_channels
    DROP COLUMN dkim_enabled,
    DROP COLUMN dkim_private_key,
    DROP COLUMN dkim_selector,
    DROP COLUMN broadcast_rate,
    DROP COLUMN broadcast_smtp_tls,
    DROP COLUMN broadcast_smtp_password,
    DROP COLUMN broadcast_smtp_username,
    DROP COLUMN broadcast_smtp_port,
    DROP COLUMN broadcast_smtp_host;
ALTER TABLE inboxes
    DROP COLUMN sender_contact,
    DROP COLUMN sender_registration,
    DROP COLUMN sender_postal_address,
    DROP COLUMN sender_legal_name;
