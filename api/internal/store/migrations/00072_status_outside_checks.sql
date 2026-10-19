-- +goose Up
ALTER TABLE status_monitors
    ADD COLUMN target            text CHECK (length(target) BETWEEN 1 AND 2048),
    ADD COLUMN timeout_seconds   integer CHECK (timeout_seconds BETWEEN 1 AND 30),
    ADD COLUMN http_method       text CHECK (http_method IN ('GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS')),
    ADD COLUMN http_header_names text[] NOT NULL DEFAULT '{}',
    -- http_headers is the JSON list of names and values sealed under the master key.
    ADD COLUMN http_headers      bytea,
    ADD COLUMN http_body         text CHECK (length(http_body) <= 16384),
    ADD COLUMN status_min        integer CHECK (status_min BETWEEN 100 AND 599),
    ADD COLUMN status_max        integer CHECK (status_max BETWEEN 100 AND 599),
    ADD COLUMN keyword           text CHECK (length(keyword) BETWEEN 1 AND 200),
    ADD COLUMN follow_redirects  boolean NOT NULL DEFAULT false,
    ADD COLUMN cert_warn_days    integer CHECK (cert_warn_days BETWEEN 0 AND 365),
    ADD COLUMN cert_expires_at   timestamptz,
    ADD COLUMN check_at          timestamptz,
    ADD CONSTRAINT status_monitors_outside CHECK (kind NOT IN ('http', 'tcp')
        OR (target IS NOT NULL AND interval_seconds IS NOT NULL AND timeout_seconds IS NOT NULL)),
    ADD CONSTRAINT status_monitors_http CHECK (kind <> 'http'
        OR (http_method IS NOT NULL AND status_min IS NOT NULL AND status_max IS NOT NULL AND status_min <= status_max AND cert_warn_days IS NOT NULL));
CREATE INDEX status_monitors_check_idx ON status_monitors (check_at) WHERE check_at IS NOT NULL AND NOT paused;

-- A pending check waits for its round to be judged before it counts; round_id names the round.
ALTER TABLE status_checks
    ADD COLUMN round_id uuid,
    ADD COLUMN pending  boolean NOT NULL DEFAULT false;
CREATE INDEX status_checks_round_idx ON status_checks (round_id) WHERE pending;

-- +goose Down
DROP INDEX status_checks_round_idx;
ALTER TABLE status_checks DROP COLUMN pending, DROP COLUMN round_id;
DROP INDEX status_monitors_check_idx;
ALTER TABLE status_monitors
    DROP CONSTRAINT status_monitors_http,
    DROP CONSTRAINT status_monitors_outside,
    DROP COLUMN check_at,
    DROP COLUMN cert_expires_at,
    DROP COLUMN cert_warn_days,
    DROP COLUMN follow_redirects,
    DROP COLUMN keyword,
    DROP COLUMN status_max,
    DROP COLUMN status_min,
    DROP COLUMN http_body,
    DROP COLUMN http_headers,
    DROP COLUMN http_header_names,
    DROP COLUMN http_method,
    DROP COLUMN timeout_seconds,
    DROP COLUMN target;
