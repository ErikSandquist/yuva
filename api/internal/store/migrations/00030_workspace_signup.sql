-- +goose Up
ALTER TABLE workspaces
    ADD COLUMN created_by uuid REFERENCES people (id) ON DELETE SET NULL,
    ADD COLUMN created_ip text;
CREATE INDEX workspaces_created_by_idx ON workspaces (created_by) WHERE created_by IS NOT NULL;
CREATE INDEX workspaces_created_ip_idx ON workspaces (created_ip, created_at) WHERE created_ip IS NOT NULL;

-- +goose Down
DROP INDEX workspaces_created_ip_idx;
DROP INDEX workspaces_created_by_idx;
ALTER TABLE workspaces DROP COLUMN created_ip, DROP COLUMN created_by;
