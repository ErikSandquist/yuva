-- +goose Up
ALTER TABLE status_incident_updates ADD COLUMN notify boolean NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE status_incident_updates DROP COLUMN notify;
