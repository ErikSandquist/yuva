-- +goose Up
ALTER TABLE email_channels ADD COLUMN dkim_public_key bytea;

-- One row per channel, replaced by each check. `timeout` is not stored: a probe still `queued` or
-- `sent` 10 minutes after created_at is reported as timed out.
CREATE TABLE deliverability_checks (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id   uuid NOT NULL,
    probe_token  text NOT NULL CHECK (probe_token ~ '^[a-z2-7]{24}$'),
    state        text NOT NULL CHECK (state IN ('queued', 'sent', 'received', 'unverifiable', 'failed')),
    result       jsonb NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL,
    finished_at  timestamptz,
    PRIMARY KEY (workspace_id, channel_id),
    UNIQUE (workspace_id, probe_token),
    FOREIGN KEY (workspace_id, channel_id) REFERENCES channels (workspace_id, id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE deliverability_checks;
ALTER TABLE email_channels DROP COLUMN dkim_public_key;
