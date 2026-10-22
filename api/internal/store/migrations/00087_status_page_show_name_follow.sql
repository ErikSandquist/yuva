-- +goose Up
ALTER TABLE status_pages
    ADD COLUMN show_name boolean NOT NULL DEFAULT true,
    ADD COLUMN show_follow boolean NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE status_pages DROP COLUMN show_follow, DROP COLUMN show_name;
