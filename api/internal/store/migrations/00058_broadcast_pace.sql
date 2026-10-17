-- +goose Up
ALTER TABLE broadcasts
    ADD COLUMN pace_per_hour  integer CHECK (pace_per_hour BETWEEN 1 AND 10000000),
    ADD COLUMN pace_per_day   integer CHECK (pace_per_day BETWEEN 1 AND 10000000),
    ADD COLUMN pace_per_month integer CHECK (pace_per_month BETWEEN 1 AND 10000000),
    ADD COLUMN pace_next_at   timestamptz;

-- +goose Down
ALTER TABLE broadcasts
    DROP COLUMN pace_per_hour,
    DROP COLUMN pace_per_day,
    DROP COLUMN pace_per_month,
    DROP COLUMN pace_next_at;
