-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE conversations ADD CONSTRAINT conversations_feedback_score_check CHECK (
    feedback IS NULL OR NOT (feedback ? 'score')
    OR (jsonb_typeof(feedback -> 'score') = 'number' AND feedback -> 'score' IN ('1', '2', '3', '4', '5'))
) NOT VALID;
ALTER TABLE conversations VALIDATE CONSTRAINT conversations_feedback_score_check;

-- +goose Down
ALTER TABLE conversations DROP CONSTRAINT conversations_feedback_score_check;
