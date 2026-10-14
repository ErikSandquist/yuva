-- +goose Up
-- The zones an API property counted its hours and days in before its current timezone, oldest
-- first: [{"timezone": "Europe/Istanbul", "until": "2026-10-15T21:00:00Z"}]. A new zone applies from
-- the next local midnight of the zone before it, so the last until can lie ahead.
ALTER TABLE analytics_properties ADD COLUMN timezone_history jsonb NOT NULL DEFAULT '[]';

-- monitor_limit_reached_at: when the workspace hit its monthly request records cap, so its owners and
-- the operator hear about it once a month.
ALTER TABLE usage_counters ADD COLUMN monitor_limit_reached_at timestamptz;

ALTER TABLE operator_notices
    DROP CONSTRAINT operator_notices_kind_check,
    ADD CONSTRAINT operator_notices_kind_check
        CHECK (kind IN ('signup', 'workspace', 'broadcast_approval', 'broadcast_suspended', 'broadcast_paused', 'analytics_limit', 'analytics_dropped',
            'monitor_limit'));

-- +goose Down
DELETE FROM operator_notices WHERE kind = 'monitor_limit';
ALTER TABLE operator_notices
    DROP CONSTRAINT operator_notices_kind_check,
    ADD CONSTRAINT operator_notices_kind_check
        CHECK (kind IN ('signup', 'workspace', 'broadcast_approval', 'broadcast_suspended', 'broadcast_paused', 'analytics_limit', 'analytics_dropped'));
ALTER TABLE usage_counters DROP COLUMN monitor_limit_reached_at;
ALTER TABLE analytics_properties DROP COLUMN timezone_history;
