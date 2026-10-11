-- +goose Up
ALTER TABLE conversations ADD COLUMN channel_kind text CHECK (channel_kind IN ('email', 'chat', 'app', 'api'));
UPDATE conversations c SET channel_kind = ch.kind
FROM channels ch
WHERE ch.workspace_id = c.workspace_id AND ch.id = c.channel_id;

-- +goose Down
ALTER TABLE conversations DROP COLUMN channel_kind;
