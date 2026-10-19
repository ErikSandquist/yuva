-- +goose Up
CREATE TABLE status_monitors (
    id                    uuid PRIMARY KEY,
    workspace_id          uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    page_id               uuid NOT NULL,
    component_id          uuid NOT NULL,
    kind                  text NOT NULL CHECK (kind IN ('heartbeat', 'push', 'http', 'tcp')),
    name                  text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    token_hash            bytea UNIQUE,
    token_prefix          text,
    interval_seconds      integer CHECK (interval_seconds BETWEEN 30 AND 2592000),
    grace_seconds         integer CHECK (grace_seconds BETWEEN 0 AND 86400),
    failure_state         text NOT NULL CHECK (failure_state IN ('degraded', 'partial_outage', 'major_outage')),
    failure_threshold     integer NOT NULL CHECK (failure_threshold BETWEEN 1 AND 10),
    recovery_threshold    integer NOT NULL CHECK (recovery_threshold BETWEEN 1 AND 10),
    on_failure            text NOT NULL CHECK (on_failure IN ('none', 'draft', 'publish')),
    paused                boolean NOT NULL DEFAULT false,
    state                 text NOT NULL CHECK (state IN ('pending', 'up', 'down')),
    down_state            text CHECK (down_state IN ('degraded', 'partial_outage', 'major_outage')),
    consecutive_failures  integer NOT NULL DEFAULT 0,
    consecutive_successes integer NOT NULL DEFAULT 0,
    last_result_at        timestamptz,
    last_message          text CHECK (length(last_message) <= 1000),
    -- due_at is when a missing heartbeat becomes a failure; due_extended marks the one extra interval
    -- given when the server itself started after the beat was expected.
    due_at                timestamptz,
    due_extended          boolean NOT NULL DEFAULT false,
    incident_id           uuid,
    changed_at            timestamptz NOT NULL,
    created_at            timestamptz NOT NULL,
    updated_at            timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    CHECK ((kind IN ('heartbeat', 'push')) = (token_hash IS NOT NULL AND token_prefix IS NOT NULL)),
    CHECK (kind <> 'heartbeat' OR (interval_seconds IS NOT NULL AND grace_seconds IS NOT NULL)),
    CHECK ((state = 'down') = (down_state IS NOT NULL)),
    FOREIGN KEY (workspace_id, page_id) REFERENCES status_pages (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, component_id) REFERENCES status_components (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, incident_id) REFERENCES status_incidents (workspace_id, id) ON DELETE SET NULL (incident_id)
);
CREATE INDEX status_monitors_page_idx ON status_monitors (workspace_id, page_id, created_at, id);
CREATE INDEX status_monitors_component_idx ON status_monitors (workspace_id, component_id);
CREATE INDEX status_monitors_due_idx ON status_monitors (workspace_id, due_at) WHERE due_at IS NOT NULL AND NOT paused;

CREATE TABLE status_checks (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    monitor_id   uuid NOT NULL,
    at           timestamptz NOT NULL,
    ok           boolean NOT NULL,
    source       text NOT NULL CHECK (source IN ('beat', 'fail', 'missed', 'push', 'alertmanager', 'grafana', 'http', 'tcp')),
    state        text CHECK (state IN ('operational', 'degraded', 'partial_outage', 'major_outage', 'maintenance')),
    message      text CHECK (length(message) <= 1000),
    error        text CHECK (length(error) <= 1000),
    duration_ms  integer,
    status_code  integer,
    FOREIGN KEY (workspace_id, monitor_id) REFERENCES status_monitors (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX status_checks_monitor_idx ON status_checks (workspace_id, monitor_id, at DESC, id DESC);
CREATE INDEX status_checks_at_idx ON status_checks (workspace_id, at);

-- +goose Down
DROP TABLE status_checks;
DROP TABLE status_monitors;
