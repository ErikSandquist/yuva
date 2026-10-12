-- +goose Up
-- Analytics groups and sorts these values by the byte, not by a language's rules: grouping a
-- day's paths and sources is then cheap, and no locale decides what counts as the same value.
ALTER TABLE analytics_visits
    ALTER COLUMN hostname TYPE text COLLATE "C",
    ALTER COLUMN entry_path TYPE text COLLATE "C",
    ALTER COLUMN exit_path TYPE text COLLATE "C",
    ALTER COLUMN source TYPE text COLLATE "C",
    ALTER COLUMN referrer_host TYPE text COLLATE "C",
    ALTER COLUMN referrer_path TYPE text COLLATE "C",
    ALTER COLUMN utm_source TYPE text COLLATE "C",
    ALTER COLUMN utm_medium TYPE text COLLATE "C",
    ALTER COLUMN utm_campaign TYPE text COLLATE "C",
    ALTER COLUMN utm_term TYPE text COLLATE "C",
    ALTER COLUMN utm_content TYPE text COLLATE "C",
    ALTER COLUMN country TYPE text COLLATE "C",
    ALTER COLUMN region TYPE text COLLATE "C",
    ALTER COLUMN city TYPE text COLLATE "C",
    ALTER COLUMN browser TYPE text COLLATE "C",
    ALTER COLUMN os TYPE text COLLATE "C",
    ALTER COLUMN language TYPE text COLLATE "C",
    ALTER COLUMN app_version TYPE text COLLATE "C";
ALTER TABLE analytics_events
    ALTER COLUMN name TYPE text COLLATE "C",
    ALTER COLUMN hostname TYPE text COLLATE "C",
    ALTER COLUMN path TYPE text COLLATE "C";
ALTER TABLE analytics_daily ALTER COLUMN value TYPE text COLLATE "C";

-- A day whose analytics events were mostly dropped (bots, floods, refused origins) is an operator
-- digest line.
ALTER TABLE operator_notices
    DROP CONSTRAINT operator_notices_kind_check,
    ADD CONSTRAINT operator_notices_kind_check
        CHECK (kind IN ('signup', 'workspace', 'broadcast_approval', 'broadcast_suspended', 'broadcast_paused', 'analytics_limit', 'analytics_dropped'));

-- +goose Down
DELETE FROM operator_notices WHERE kind = 'analytics_dropped';
ALTER TABLE operator_notices
    DROP CONSTRAINT operator_notices_kind_check,
    ADD CONSTRAINT operator_notices_kind_check
        CHECK (kind IN ('signup', 'workspace', 'broadcast_approval', 'broadcast_suspended', 'broadcast_paused', 'analytics_limit'));
ALTER TABLE analytics_daily ALTER COLUMN value TYPE text COLLATE "default";
ALTER TABLE analytics_events
    ALTER COLUMN name TYPE text COLLATE "default",
    ALTER COLUMN hostname TYPE text COLLATE "default",
    ALTER COLUMN path TYPE text COLLATE "default";
ALTER TABLE analytics_visits
    ALTER COLUMN hostname TYPE text COLLATE "default",
    ALTER COLUMN entry_path TYPE text COLLATE "default",
    ALTER COLUMN exit_path TYPE text COLLATE "default",
    ALTER COLUMN source TYPE text COLLATE "default",
    ALTER COLUMN referrer_host TYPE text COLLATE "default",
    ALTER COLUMN referrer_path TYPE text COLLATE "default",
    ALTER COLUMN utm_source TYPE text COLLATE "default",
    ALTER COLUMN utm_medium TYPE text COLLATE "default",
    ALTER COLUMN utm_campaign TYPE text COLLATE "default",
    ALTER COLUMN utm_term TYPE text COLLATE "default",
    ALTER COLUMN utm_content TYPE text COLLATE "default",
    ALTER COLUMN country TYPE text COLLATE "default",
    ALTER COLUMN region TYPE text COLLATE "default",
    ALTER COLUMN city TYPE text COLLATE "default",
    ALTER COLUMN browser TYPE text COLLATE "default",
    ALTER COLUMN os TYPE text COLLATE "default",
    ALTER COLUMN language TYPE text COLLATE "default",
    ALTER COLUMN app_version TYPE text COLLATE "default";
