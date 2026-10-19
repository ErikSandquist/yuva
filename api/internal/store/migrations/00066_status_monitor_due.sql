-- +goose Up
DROP INDEX status_monitors_due_idx;
CREATE INDEX status_monitors_due_idx ON status_monitors (due_at) WHERE due_at IS NOT NULL AND NOT paused;

-- +goose Down
DROP INDEX status_monitors_due_idx;
CREATE INDEX status_monitors_due_idx ON status_monitors (workspace_id, due_at) WHERE due_at IS NOT NULL AND NOT paused;
