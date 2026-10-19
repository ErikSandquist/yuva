-- +goose Up
-- round_dropped is the round's judgement, stored on its pending checks before any is applied.
ALTER TABLE status_checks ADD COLUMN round_dropped boolean;

-- +goose Down
ALTER TABLE status_checks DROP COLUMN round_dropped;
