-- +goose Up
ALTER TABLE conversations ADD COLUMN feedback_progress text
    CHECK (feedback_progress IN ('planned', 'in_progress', 'done', 'wont_do'));

-- +goose Down
ALTER TABLE conversations DROP COLUMN feedback_progress;
