-- +goose Up
-- Uptime checks (M16.4): the server calls a check's URL on its interval through the webhook dialer
-- and records each counted run. Header values are sealed under the master key; host is the URL's
-- host name in lower case, for the 10 checks per host cap. folded_until is the last UTC day whose
-- runs went into monitor_check_days.
CREATE TABLE monitor_checks (
    id                   uuid PRIMARY KEY,
    workspace_id         uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    property_id          uuid,
    name                 text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    url                  text NOT NULL CHECK (length(url) <= 2000),
    host                 text NOT NULL,
    method               text NOT NULL CHECK (method IN ('GET', 'HEAD')),
    headers              jsonb NOT NULL DEFAULT '[]',
    expected_statuses    integer[] NOT NULL DEFAULT '{}' CHECK (cardinality(expected_statuses) <= 20 AND 200 <= ALL (expected_statuses) AND 399 >= ALL (expected_statuses)),
    keyword              text CHECK (length(keyword) BETWEEN 1 AND 200),
    keyword_absent       boolean NOT NULL DEFAULT false,
    timeout_seconds      integer NOT NULL CHECK (timeout_seconds BETWEEN 1 AND 30),
    interval_seconds     integer NOT NULL CHECK (interval_seconds BETWEEN 30 AND 86400),
    failures_before_down integer NOT NULL CHECK (failures_before_down BETWEEN 1 AND 10),
    cert_warning_days    integer NOT NULL CHECK (cert_warning_days BETWEEN 1 AND 90),
    max_redirects        integer NOT NULL CHECK (max_redirects BETWEEN 0 AND 5),
    service              text CHECK (length(service) BETWEEN 1 AND 100),
    paused               boolean NOT NULL DEFAULT false,
    state                text NOT NULL DEFAULT 'unknown' CHECK (state IN ('unknown', 'up', 'down')),
    state_since          timestamptz,
    failures_in_row      integer NOT NULL DEFAULT 0,
    next_run_at          timestamptz NOT NULL,
    last_run_at          timestamptz,
    cert_expires_at      timestamptz,
    folded_until         date,
    created_by           uuid,
    created_at           timestamptz NOT NULL,
    updated_at           timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE SET NULL (property_id)
);
CREATE INDEX monitor_checks_workspace_idx ON monitor_checks (workspace_id, name);
CREATE INDEX monitor_checks_host_idx ON monitor_checks (workspace_id, host);
CREATE INDEX monitor_checks_due_idx ON monitor_checks (next_run_at) WHERE NOT paused;

-- One row per counted run: a failed try is tried again 20 seconds later and only that second try
-- is kept. state is NULL until the run's minute is judged (unknown when most of the server's checks
-- failed in it), then the check's state after the run. Times are microseconds; seconds is the
-- check's interval, the time the run stands for. Day partitions hold every workspace's runs, made
-- and dropped by the runner and the analytics_partitions job; no foreign keys, a check's rows are
-- deleted with it.
CREATE TABLE monitor_check_results (
    workspace_id   uuid NOT NULL,
    check_id       uuid NOT NULL,
    at             timestamptz NOT NULL,
    ok             boolean NOT NULL,
    state          text CHECK (state IN ('up', 'down', 'unknown')),
    status         integer NOT NULL,
    error          text,
    detail         text,
    retried        boolean NOT NULL,
    redirects      integer NOT NULL,
    dns_us         bigint NOT NULL,
    connect_us     bigint NOT NULL,
    tls_us         bigint NOT NULL,
    ttfb_us        bigint NOT NULL,
    total_us       bigint NOT NULL,
    answered       boolean NOT NULL,
    cert_days_left integer,
    seconds        integer NOT NULL,
    PRIMARY KEY (check_id, at)
) PARTITION BY RANGE (at);
CREATE INDEX monitor_check_results_pending_idx ON monitor_check_results (at) WHERE state IS NULL;
CREATE INDEX monitor_check_results_minute_idx ON monitor_check_results (at);

-- A check's UTC day folded from its runs, kept 400 days: monitored and down seconds, the total
-- times of answered runs as a sketch, and the last run's outcome and certificate.
CREATE TABLE monitor_check_days (
    workspace_id      uuid NOT NULL,
    check_id          uuid NOT NULL,
    day               date NOT NULL,
    runs              bigint NOT NULL,
    failures          bigint NOT NULL,
    unknown           bigint NOT NULL,
    monitored_seconds bigint NOT NULL,
    down_seconds      bigint NOT NULL,
    answered          bigint NOT NULL,
    total_sum_us      bigint NOT NULL,
    ttfb_sum_us       bigint NOT NULL,
    durations         bytea NOT NULL,
    last_at           timestamptz,
    last_ok           boolean,
    cert_at           timestamptz,
    cert_days_left    integer,
    PRIMARY KEY (check_id, day),
    FOREIGN KEY (workspace_id, check_id) REFERENCES monitor_checks (workspace_id, id) ON DELETE CASCADE
);

-- Alert rules of API monitoring on a property (narrowed by route or service) or on checks.
-- recipients are member ids; threshold is null for the kinds without one.
CREATE TABLE monitor_alert_rules (
    id             uuid PRIMARY KEY,
    workspace_id   uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name           text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    kind           text NOT NULL CHECK (kind IN ('error_rate', 'client_error_rate', 'p95', 'p99', 'apdex', 'spike', 'drop', 'silent', 'check_state', 'certificate')),
    property_id    uuid,
    route          text COLLATE "C" CHECK (length(route) BETWEEN 1 AND 300),
    service        text COLLATE "C" CHECK (length(service) BETWEEN 1 AND 100),
    check_id       uuid,
    threshold      double precision,
    window_minutes integer NOT NULL DEFAULT 5 CHECK (window_minutes BETWEEN 5 AND 60),
    min_requests   integer NOT NULL DEFAULT 20 CHECK (min_requests BETWEEN 1 AND 1000000),
    recipients     uuid[] NOT NULL DEFAULT '{}' CHECK (cardinality(recipients) <= 20),
    enabled        boolean NOT NULL DEFAULT true,
    created_by     uuid,
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, check_id) REFERENCES monitor_checks (workspace_id, id) ON DELETE CASCADE,
    CHECK (kind IN ('check_state', 'certificate') OR (property_id IS NOT NULL AND check_id IS NULL)),
    CHECK (kind NOT IN ('check_state', 'certificate') OR (route IS NULL AND service IS NULL))
);
CREATE INDEX monitor_alert_rules_workspace_idx ON monitor_alert_rules (workspace_id, created_at);

-- An alert is open (resolved_at NULL) for at most one row per rule and key: the rule's subject, a
-- route of a silent rule or a check of a check rule. fired_at is the end of the window that fired.
CREATE TABLE monitor_alerts (
    id             uuid PRIMARY KEY,
    workspace_id   uuid NOT NULL,
    rule_id        uuid NOT NULL,
    key            text COLLATE "C" NOT NULL,
    kind           text NOT NULL,
    property_id    uuid,
    check_id       uuid,
    value          double precision NOT NULL,
    threshold      double precision,
    baseline       double precision,
    resolved_value double precision,
    window_minutes integer NOT NULL,
    text           text NOT NULL,
    fired_at       timestamptz NOT NULL,
    resolved_at    timestamptz,
    annotation_id  uuid,
    created_at     timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, rule_id) REFERENCES monitor_alert_rules (workspace_id, id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX monitor_alerts_open_idx ON monitor_alerts (rule_id, key) WHERE resolved_at IS NULL;
CREATE INDEX monitor_alerts_workspace_idx ON monitor_alerts (workspace_id, fired_at DESC);

-- +goose Down
DROP TABLE monitor_alerts;
DROP TABLE monitor_alert_rules;
DROP TABLE monitor_check_days;
DROP TABLE monitor_check_results;
DROP TABLE monitor_checks;
