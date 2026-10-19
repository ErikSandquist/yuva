-- +goose Up
ALTER TABLE status_pages ADD COLUMN show_response_times boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE status_pages DROP COLUMN show_response_times;
