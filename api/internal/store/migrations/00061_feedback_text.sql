-- +goose Up
UPDATE conversations c SET feedback = c.feedback || jsonb_build_object(
    'text', coalesce((
        SELECT btrim(left(btrim(split_part(btrim(m.body, E' \t\r\n\f\v'), E'\n', 1), E' \t\r\n\f\v'), 200), E' \t\r\n\f\v')
        FROM messages m
        WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id
          AND m.kind = 'message' AND NOT m.draft
        ORDER BY m.created_at, m.id
        LIMIT 1), ''),
    'attachment_count', coalesce((
        SELECT count(*) FROM attachments a
        WHERE a.workspace_id = c.workspace_id AND a.message_id = (
            SELECT m.id FROM messages m
            WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id
              AND m.kind = 'message' AND NOT m.draft
            ORDER BY m.created_at, m.id
            LIMIT 1)), 0))
WHERE c.kind = 'feedback' AND c.feedback IS NOT NULL;

-- +goose Down
UPDATE conversations SET feedback = feedback - 'text' - 'attachment_count'
WHERE kind = 'feedback' AND feedback IS NOT NULL;
