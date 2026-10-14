-- +goose Up
-- Service health (M16.7): a service is made by its first health report, by a member, or by the
-- first request record or span that names it; name and environment are unique per workspace.
-- better_in_row counts the evaluations in a row that found the same better state, better_state,
-- which applies at two. silence_seconds NULL is three intervals; last_message is why the last
-- report's own status was not healthy (a pull that got no answer).
CREATE TABLE monitor_services (
    id                   uuid PRIMARY KEY,
    workspace_id         uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name                 text COLLATE "C" NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    environment          text COLLATE "C" NOT NULL CHECK (length(environment) BETWEEN 1 AND 40),
    origin               text NOT NULL CHECK (origin IN ('report', 'member', 'records')),
    property_id          uuid,
    mode                 text NOT NULL DEFAULT 'push' CHECK (mode IN ('push', 'pull')),
    health_url           text CHECK (length(health_url) <= 2000),
    interval_seconds     integer NOT NULL DEFAULT 60 CHECK (interval_seconds BETWEEN 30 AND 600),
    timeout_seconds      integer NOT NULL DEFAULT 10 CHECK (timeout_seconds BETWEEN 1 AND 30),
    silence_seconds      integer CHECK (silence_seconds BETWEEN 60 AND 86400),
    error_rate_threshold double precision NOT NULL DEFAULT 5 CHECK (error_rate_threshold > 0 AND error_rate_threshold <= 100),
    p95_threshold_ms     double precision CHECK (p95_threshold_ms > 0 AND p95_threshold_ms <= 3600000),
    version              text CHECK (length(version) <= 80),
    state                text NOT NULL DEFAULT 'unknown' CHECK (state IN ('unknown', 'healthy', 'degraded', 'down')),
    state_since          timestamptz,
    reason               text,
    better_state         text,
    better_in_row        integer NOT NULL DEFAULT 0,
    last_report_at       timestamptz,
    last_status          text CHECK (last_status IN ('healthy', 'degraded', 'down')),
    last_message         text CHECK (length(last_message) <= 200),
    next_pull_at         timestamptz,
    created_by           uuid,
    created_at           timestamptz NOT NULL,
    updated_at           timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, name, environment),
    CHECK (mode = 'push' OR health_url IS NOT NULL),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE SET NULL (property_id)
);
CREATE INDEX monitor_services_pull_idx ON monitor_services (next_pull_at) WHERE mode = 'pull';

-- A service's dependencies as its reports name them, 50 per service. status, latency_ms and
-- message are the last report's; reported_at is the report that last listed it.
CREATE TABLE monitor_dependencies (
    id             uuid PRIMARY KEY,
    workspace_id   uuid NOT NULL,
    service_id     uuid NOT NULL,
    name           text COLLATE "C" NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    kind           text NOT NULL CHECK (kind IN ('database', 'cache', 'queue', 'http', 'other')),
    critical       boolean NOT NULL,
    status         text CHECK (status IN ('healthy', 'degraded', 'down')),
    latency_ms     double precision,
    message        text CHECK (length(message) <= 200),
    reported_at    timestamptz,
    state          text NOT NULL DEFAULT 'unknown' CHECK (state IN ('unknown', 'healthy', 'degraded', 'down')),
    state_since    timestamptz,
    better_state   text,
    better_in_row  integer NOT NULL DEFAULT 0,
    created_at     timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (service_id, name),
    FOREIGN KEY (workspace_id, service_id) REFERENCES monitor_services (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX monitor_dependencies_service_idx ON monitor_dependencies (workspace_id, service_id);

-- The processes that report for a service, 200 per service, deleted after 24 hours of silence.
CREATE TABLE monitor_instances (
    workspace_id  uuid NOT NULL,
    service_id    uuid NOT NULL,
    instance      text COLLATE "C" NOT NULL CHECK (length(instance) BETWEEN 1 AND 200),
    version       text CHECK (length(version) <= 80),
    status        text CHECK (status IN ('healthy', 'degraded', 'down')),
    first_seen_at timestamptz NOT NULL,
    last_seen_at  timestamptz NOT NULL,
    PRIMARY KEY (service_id, instance),
    FOREIGN KEY (workspace_id, service_id) REFERENCES monitor_services (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX monitor_instances_seen_idx ON monitor_instances (workspace_id, last_seen_at);

-- Every state change of a service or one of its dependencies, kept 400 days; history and uptime
-- per day are computed from them. maintenance marks a change inside an active window.
CREATE TABLE monitor_state_changes (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL,
    service_id    uuid NOT NULL,
    dependency_id uuid,
    state         text NOT NULL CHECK (state IN ('unknown', 'healthy', 'degraded', 'down')),
    previous      text NOT NULL CHECK (previous IN ('unknown', 'healthy', 'degraded', 'down')),
    reason        text,
    maintenance   boolean NOT NULL,
    at            timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, service_id) REFERENCES monitor_services (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, dependency_id) REFERENCES monitor_dependencies (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX monitor_state_changes_subject_idx ON monitor_state_changes (workspace_id, service_id, dependency_id, at);
CREATE INDEX monitor_state_changes_at_idx ON monitor_state_changes (workspace_id, at);

-- Reports per UTC hour by status, for the service (dependency '') and each dependency with a
-- latency sketch at the duration scale; kept 90 days.
CREATE TABLE monitor_health_hours (
    workspace_id uuid NOT NULL,
    service_id   uuid NOT NULL,
    dependency   text COLLATE "C" NOT NULL,
    hour         timestamptz NOT NULL,
    reports      bigint NOT NULL,
    healthy      bigint NOT NULL,
    degraded     bigint NOT NULL,
    down         bigint NOT NULL,
    latency_sum  double precision NOT NULL DEFAULT 0,
    latency_n    bigint NOT NULL DEFAULT 0,
    latencies    bytea,
    PRIMARY KEY (service_id, dependency, hour),
    FOREIGN KEY (workspace_id, service_id) REFERENCES monitor_services (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX monitor_health_hours_hour_idx ON monitor_health_hours (workspace_id, hour);

-- Incidents and maintenance windows. An automatic incident names its service in auto_service_id,
-- and one is open per service. healthy_since is set while that service is healthy, so the
-- incident resolves 5 minutes later.
CREATE TABLE monitor_incidents (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    origin          text NOT NULL CHECK (origin IN ('auto', 'manual')),
    kind            text NOT NULL CHECK (kind IN ('incident', 'maintenance')),
    title           text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    state           text NOT NULL CHECK (state IN ('investigating', 'identified', 'monitoring', 'resolved')),
    impact          text NOT NULL CHECK (impact IN ('none', 'minor', 'major', 'critical')),
    service_ids     uuid[] NOT NULL DEFAULT '{}' CHECK (cardinality(service_ids) <= 100),
    dependency_ids  uuid[] NOT NULL DEFAULT '{}' CHECK (cardinality(dependency_ids) <= 100),
    check_ids       uuid[] NOT NULL DEFAULT '{}' CHECK (cardinality(check_ids) <= 100),
    auto_service_id uuid,
    started_at      timestamptz NOT NULL,
    resolved_at     timestamptz,
    scheduled_start timestamptz,
    scheduled_end   timestamptz,
    published       boolean NOT NULL DEFAULT false,
    created_by      uuid,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    CHECK (kind = 'incident' OR (scheduled_start IS NOT NULL AND scheduled_end > scheduled_start)),
    CHECK ((origin = 'auto') = (auto_service_id IS NOT NULL))
);
CREATE UNIQUE INDEX monitor_incidents_auto_idx ON monitor_incidents (auto_service_id) WHERE resolved_at IS NULL;
CREATE INDEX monitor_incidents_workspace_idx ON monitor_incidents (workspace_id, started_at DESC);
CREATE INDEX monitor_incidents_open_idx ON monitor_incidents (workspace_id) WHERE resolved_at IS NULL;

CREATE TABLE monitor_incident_updates (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    incident_id  uuid NOT NULL,
    state        text NOT NULL CHECK (state IN ('investigating', 'identified', 'monitoring', 'resolved')),
    text         text NOT NULL CHECK (length(text) BETWEEN 1 AND 5000),
    author_id    uuid,
    notify       boolean NOT NULL,
    created_at   timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, incident_id) REFERENCES monitor_incidents (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX monitor_incident_updates_incident_idx ON monitor_incident_updates (workspace_id, incident_id, created_at DESC);

-- service_state rules name a service (and optionally one of its dependencies), a property's
-- services, or every service; their alerts carry the service.
ALTER TABLE monitor_alert_rules
    ADD COLUMN service_id uuid,
    ADD COLUMN dependency_id uuid,
    DROP CONSTRAINT monitor_alert_rules_kind_check,
    DROP CONSTRAINT monitor_alert_rules_check,
    DROP CONSTRAINT monitor_alert_rules_check1,
    ADD CONSTRAINT monitor_alert_rules_kind_check CHECK (kind IN ('error_rate', 'client_error_rate', 'p95', 'p99', 'apdex', 'spike', 'drop', 'silent',
        'check_state', 'certificate', 'service_state')),
    ADD CONSTRAINT monitor_alert_rules_check CHECK (kind IN ('check_state', 'certificate', 'service_state') OR (property_id IS NOT NULL AND check_id IS NULL)),
    ADD CONSTRAINT monitor_alert_rules_check1 CHECK (kind NOT IN ('check_state', 'certificate', 'service_state') OR (route IS NULL AND service IS NULL)),
    ADD CONSTRAINT monitor_alert_rules_service_check1 CHECK (kind = 'service_state' OR (service_id IS NULL AND dependency_id IS NULL)),
    ADD CONSTRAINT monitor_alert_rules_service_check2 CHECK (kind <> 'service_state' OR check_id IS NULL),
    ADD CONSTRAINT monitor_alert_rules_service_check3 CHECK (dependency_id IS NULL OR service_id IS NOT NULL),
    ADD FOREIGN KEY (workspace_id, service_id) REFERENCES monitor_services (workspace_id, id) ON DELETE CASCADE,
    ADD FOREIGN KEY (workspace_id, dependency_id) REFERENCES monitor_dependencies (workspace_id, id) ON DELETE CASCADE;

ALTER TABLE monitor_alerts
    ADD COLUMN service_id uuid,
    ADD COLUMN dependency_id uuid;

-- +goose Down
DELETE FROM monitor_alert_rules WHERE kind = 'service_state';
ALTER TABLE monitor_alerts
    DROP COLUMN dependency_id,
    DROP COLUMN service_id;
ALTER TABLE monitor_alert_rules
    DROP CONSTRAINT monitor_alert_rules_service_check1,
    DROP CONSTRAINT monitor_alert_rules_service_check2,
    DROP CONSTRAINT monitor_alert_rules_service_check3,
    DROP COLUMN dependency_id,
    DROP COLUMN service_id,
    DROP CONSTRAINT monitor_alert_rules_kind_check,
    DROP CONSTRAINT monitor_alert_rules_check,
    DROP CONSTRAINT monitor_alert_rules_check1,
    ADD CONSTRAINT monitor_alert_rules_kind_check CHECK (kind IN ('error_rate', 'client_error_rate', 'p95', 'p99', 'apdex', 'spike', 'drop', 'silent',
        'check_state', 'certificate')),
    ADD CONSTRAINT monitor_alert_rules_check CHECK (kind IN ('check_state', 'certificate') OR (property_id IS NOT NULL AND check_id IS NULL)),
    ADD CONSTRAINT monitor_alert_rules_check1 CHECK (kind NOT IN ('check_state', 'certificate') OR (route IS NULL AND service IS NULL));
DROP TABLE monitor_incident_updates;
DROP TABLE monitor_incidents;
DROP TABLE monitor_health_hours;
DROP TABLE monitor_state_changes;
DROP TABLE monitor_instances;
DROP TABLE monitor_dependencies;
DROP TABLE monitor_services;
