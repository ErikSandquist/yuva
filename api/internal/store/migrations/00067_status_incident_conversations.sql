-- +goose Up
CREATE TABLE status_incident_conversations (
    workspace_id         uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    incident_id          uuid NOT NULL,
    conversation_id      uuid NOT NULL,
    linked_by_member_id  uuid,
    linked_by_api_key_id uuid,
    linked_at            timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, incident_id, conversation_id),
    FOREIGN KEY (workspace_id, incident_id) REFERENCES status_incidents (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, conversation_id) REFERENCES conversations (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX status_incident_conversations_incident_idx ON status_incident_conversations (workspace_id, incident_id, linked_at DESC, conversation_id DESC);
CREATE INDEX status_incident_conversations_conversation_idx ON status_incident_conversations (workspace_id, conversation_id, linked_at DESC);

-- +goose Down
DROP TABLE status_incident_conversations;
