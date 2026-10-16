-- +goose Up
ALTER TABLE contacts ADD COLUMN created_by uuid;

-- +goose Down
ALTER TABLE contacts DROP COLUMN created_by;
