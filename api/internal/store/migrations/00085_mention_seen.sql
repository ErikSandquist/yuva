-- +goose Up
CREATE TABLE mention_seen (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    member_id    uuid NOT NULL,
    message_id   uuid NOT NULL,
    seen_at      timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, member_id, message_id),
    FOREIGN KEY (workspace_id, member_id) REFERENCES members (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, message_id) REFERENCES messages (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX mention_seen_message_idx ON mention_seen (workspace_id, message_id);

CREATE VIEW member_mentions AS
SELECT m.workspace_id, mm.member_id, m.id AS message_id, m.conversation_id, c.inbox_id, m.created_at, m.mentions,
       (ms.message_id IS NOT NULL
        OR (r.last_read_at IS NOT NULL AND (r.last_read_at, r.last_read_message_id) >= (m.created_at, m.id)))::bool AS seen
FROM messages m
CROSS JOIN LATERAL unnest(m.mentions) AS mm(member_id)
JOIN conversations c ON c.workspace_id = m.workspace_id AND c.id = m.conversation_id
LEFT JOIN conversation_reads r
       ON r.workspace_id = m.workspace_id AND r.conversation_id = m.conversation_id AND r.member_id = mm.member_id
LEFT JOIN mention_seen ms
       ON ms.workspace_id = m.workspace_id AND ms.message_id = m.id AND ms.member_id = mm.member_id
WHERE m.kind = 'note' AND NOT m.draft AND m.author_member_id IS DISTINCT FROM mm.member_id;

-- +goose Down
DROP VIEW member_mentions;
DROP TABLE mention_seen;
