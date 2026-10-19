-- +goose Up
CREATE TABLE status_incident_replies (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid NOT NULL,
    incident_id  uuid NOT NULL,
    body         text NOT NULL,
    close        boolean NOT NULL,
    member_id    uuid,
    api_key_id   uuid,
    grant_id     uuid,
    via          text NOT NULL DEFAULT '',
    mcp_only     boolean NOT NULL DEFAULT false,
    queued       integer NOT NULL,
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, id),
    FOREIGN KEY (workspace_id, incident_id) REFERENCES status_incidents (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX status_incident_replies_incident_idx ON status_incident_replies (workspace_id, incident_id, created_at DESC);

CREATE TABLE status_incident_reply_targets (
    workspace_id    uuid NOT NULL,
    reply_id        uuid NOT NULL,
    conversation_id uuid NOT NULL,
    outcome         text NOT NULL DEFAULT 'pending' CHECK (outcome IN ('pending', 'sent', 'skipped', 'failed')),
    reason          text,
    finished_at     timestamptz,
    PRIMARY KEY (workspace_id, reply_id, conversation_id),
    FOREIGN KEY (workspace_id, reply_id) REFERENCES status_incident_replies (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, conversation_id) REFERENCES conversations (workspace_id, id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE status_incident_reply_targets;
DROP TABLE status_incident_replies;
