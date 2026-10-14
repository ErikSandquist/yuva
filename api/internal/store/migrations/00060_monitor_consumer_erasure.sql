-- +goose Up
-- An erased consumer (M16.5) keeps its row without a value, so its id is never handed to another
-- consumer while a process may still write rows under it; records naming it later get a new id.
ALTER TABLE monitor_names
    ALTER COLUMN value DROP NOT NULL,
    ADD COLUMN erased_at timestamptz,
    ADD CONSTRAINT monitor_names_erased_check CHECK ((value IS NULL) = (erased_at IS NOT NULL) AND (value IS NOT NULL OR kind = 'consumer'));

-- +goose Down
DELETE FROM monitor_names WHERE value IS NULL;
ALTER TABLE monitor_names
    DROP CONSTRAINT monitor_names_erased_check,
    DROP COLUMN erased_at,
    ALTER COLUMN value SET NOT NULL;
