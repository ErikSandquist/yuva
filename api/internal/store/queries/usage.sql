-- name: AddUsage :exec
INSERT INTO usage_counters (workspace_id, month, conversations, messages, attachment_bytes)
VALUES (@workspace_id, @month, @conversations, @messages, @attachment_bytes)
ON CONFLICT (workspace_id, month) DO UPDATE SET
    conversations = usage_counters.conversations + EXCLUDED.conversations,
    messages = usage_counters.messages + EXCLUDED.messages,
    attachment_bytes = usage_counters.attachment_bytes + EXCLUDED.attachment_bytes;

-- name: AddImportedUsage :exec
INSERT INTO usage_counters (workspace_id, month, imported_conversations, imported_messages, imported_bytes)
VALUES (@workspace_id, @month, @conversations, @messages, @bytes)
ON CONFLICT (workspace_id, month) DO UPDATE SET
    imported_conversations = usage_counters.imported_conversations + EXCLUDED.imported_conversations,
    imported_messages = usage_counters.imported_messages + EXCLUDED.imported_messages,
    imported_bytes = usage_counters.imported_bytes + EXCLUDED.imported_bytes;

-- name: ListUsage :many
SELECT * FROM usage_counters WHERE workspace_id = $1 ORDER BY month DESC LIMIT 24;
