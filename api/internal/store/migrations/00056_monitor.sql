-- +goose Up
-- API monitoring (M16.1). An analytics property of kind api watches an API: its host sends a record
-- per request, which each process adds up in memory and writes as rows per minute and hour, never a
-- row per request and never an address. Its domains (0 to 50) are the API's host names and its
-- platforms the apps that may send app-side records; it has no goals, props or consent features.
ALTER TABLE analytics_properties
    ADD COLUMN kind text NOT NULL DEFAULT 'site' CHECK (kind IN ('site', 'api')),
    ADD COLUMN apdex_ms integer NOT NULL DEFAULT 500 CHECK (apdex_ms BETWEEN 1 AND 60000),
    ADD COLUMN monitor_hour_days integer NOT NULL DEFAULT 90 CHECK (monitor_hour_days BETWEEN 7 AND 400),
    DROP CONSTRAINT analytics_properties_domains_check,
    ADD CONSTRAINT analytics_properties_domains_check CHECK (cardinality(domains) <= 50 AND (kind = 'api' OR cardinality(domains) >= 1)),
    DROP CONSTRAINT analytics_properties_platforms_check,
    ADD CONSTRAINT analytics_properties_platforms_check CHECK (platforms <@ ARRAY['web', 'ios', 'android']
        AND CASE WHEN kind = 'api' THEN NOT ('web' = ANY (platforms)) ELSE cardinality(platforms) >= 1 END),
    ADD CONSTRAINT analytics_properties_api_check CHECK (kind = 'site' OR (consent_features = '{}' AND tracked_props = '{}'));

-- Dictionaries give the rows integer ids. A route is a pattern such as /users/{id}; inferred ones
-- came from a path. (other) and (unmatched) are rows too but do not count towards the caps.
CREATE TABLE monitor_routes (
    workspace_id  uuid NOT NULL,
    property_id   uuid NOT NULL,
    id            integer NOT NULL,
    pattern       text COLLATE "C" NOT NULL,
    inferred      boolean NOT NULL DEFAULT false,
    hidden        boolean NOT NULL DEFAULT false,
    apdex_ms      integer CHECK (apdex_ms BETWEEN 1 AND 60000),
    first_seen_at timestamptz NOT NULL,
    last_seen_at  timestamptz NOT NULL,
    PRIMARY KEY (property_id, id),
    UNIQUE (property_id, pattern),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

-- Consumers, API versions, app versions, environments, services and countries of a property. Id 0
-- is the empty value and has no row.
CREATE TABLE monitor_names (
    workspace_id  uuid NOT NULL,
    property_id   uuid NOT NULL,
    kind          text NOT NULL CHECK (kind IN ('environment', 'service', 'api_version', 'consumer', 'app_version', 'country')),
    id            integer NOT NULL CHECK (id > 0),
    value         text COLLATE "C" NOT NULL,
    first_seen_at timestamptz NOT NULL,
    PRIMARY KEY (property_id, kind, id),
    UNIQUE (property_id, kind, value),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX monitor_names_services_idx ON monitor_names (workspace_id, kind, value);

-- Core rows by UTC minute, one per process (writer) and key. A process upserts its cumulative totals
-- every 10 seconds, replacing its previous row, so processes never share a row. durations and
-- response_sizes are sketches (log-bucketed histograms) that only Go opens. Day partitions, made
-- and dropped by the aggregator and the analytics_partitions job; no foreign keys, a property's rows
-- are deleted explicitly.
CREATE TABLE monitor_minute (
    workspace_id   uuid NOT NULL,
    property_id    uuid NOT NULL,
    minute         timestamptz NOT NULL,
    writer         bigint NOT NULL,
    environment    integer NOT NULL,
    service        integer NOT NULL,
    route          integer NOT NULL,
    method         smallint NOT NULL,
    status         smallint NOT NULL,
    api_version    integer NOT NULL,
    source         smallint NOT NULL,
    platform       smallint NOT NULL,
    requests       bigint NOT NULL,
    errors         bigint NOT NULL,
    client_errors  bigint NOT NULL,
    sampled        bigint NOT NULL,
    duration_sum   bigint NOT NULL,
    duration_min   bigint NOT NULL,
    duration_max   bigint NOT NULL,
    durations      bytea NOT NULL,
    request_bytes  bigint NOT NULL,
    response_bytes bigint NOT NULL,
    response_sizes bytea NOT NULL,
    PRIMARY KEY (property_id, minute, writer, environment, service, route, method, status, api_version, source, platform)
) PARTITION BY RANGE (minute);

-- Core rows by the property's local hour (hour is the hour's first instant), folded from the minute
-- rows once the hour is closed. Month partitions; rows past the property's monitor_hour_days are
-- deleted.
CREATE TABLE monitor_hour (
    workspace_id   uuid NOT NULL,
    property_id    uuid NOT NULL,
    hour           timestamptz NOT NULL,
    environment    integer NOT NULL,
    service        integer NOT NULL,
    route          integer NOT NULL,
    method         smallint NOT NULL,
    status         smallint NOT NULL,
    api_version    integer NOT NULL,
    source         smallint NOT NULL,
    platform       smallint NOT NULL,
    requests       bigint NOT NULL,
    errors         bigint NOT NULL,
    client_errors  bigint NOT NULL,
    sampled        bigint NOT NULL,
    duration_sum   bigint NOT NULL,
    duration_min   bigint NOT NULL,
    duration_max   bigint NOT NULL,
    durations      bytea NOT NULL,
    request_bytes  bigint NOT NULL,
    response_bytes bigint NOT NULL,
    response_sizes bytea NOT NULL,
    PRIMARY KEY (property_id, hour, environment, service, route, method, status, api_version, source, platform)
) PARTITION BY RANGE (hour);

-- The extra dimensions (dim 1 consumer, 2 country, 3 app version) by local hour, each with route,
-- method and status class (0 for no response, else the status's hundreds). Written per writer like
-- the minute rows and compacted to writer 0 when the hour is folded.
CREATE TABLE monitor_dim_hour (
    workspace_id   uuid NOT NULL,
    property_id    uuid NOT NULL,
    hour           timestamptz NOT NULL,
    writer         bigint NOT NULL,
    dim            smallint NOT NULL,
    value          integer NOT NULL,
    route          integer NOT NULL,
    method         smallint NOT NULL,
    status_class   smallint NOT NULL,
    requests       bigint NOT NULL,
    errors         bigint NOT NULL,
    client_errors  bigint NOT NULL,
    sampled        bigint NOT NULL,
    duration_sum   bigint NOT NULL,
    duration_min   bigint NOT NULL,
    duration_max   bigint NOT NULL,
    durations      bytea NOT NULL,
    request_bytes  bigint NOT NULL,
    response_bytes bigint NOT NULL,
    response_sizes bytea NOT NULL,
    PRIMARY KEY (property_id, hour, writer, dim, value, route, method, status_class)
) PARTITION BY RANGE (hour);

-- Local days folded from the hours, kept the property's retention_months.
CREATE TABLE monitor_day (
    workspace_id   uuid NOT NULL,
    property_id    uuid NOT NULL,
    day            date NOT NULL,
    environment    integer NOT NULL,
    service        integer NOT NULL,
    route          integer NOT NULL,
    method         smallint NOT NULL,
    status         smallint NOT NULL,
    api_version    integer NOT NULL,
    source         smallint NOT NULL,
    platform       smallint NOT NULL,
    requests       bigint NOT NULL,
    errors         bigint NOT NULL,
    client_errors  bigint NOT NULL,
    sampled        bigint NOT NULL,
    duration_sum   bigint NOT NULL,
    duration_min   bigint NOT NULL,
    duration_max   bigint NOT NULL,
    durations      bytea NOT NULL,
    request_bytes  bigint NOT NULL,
    response_bytes bigint NOT NULL,
    response_sizes bytea NOT NULL,
    PRIMARY KEY (property_id, day, environment, service, route, method, status, api_version, source, platform),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE monitor_dim_day (
    workspace_id   uuid NOT NULL,
    property_id    uuid NOT NULL,
    day            date NOT NULL,
    dim            smallint NOT NULL,
    value          integer NOT NULL,
    route          integer NOT NULL,
    method         smallint NOT NULL,
    status_class   smallint NOT NULL,
    requests       bigint NOT NULL,
    errors         bigint NOT NULL,
    client_errors  bigint NOT NULL,
    sampled        bigint NOT NULL,
    duration_sum   bigint NOT NULL,
    duration_min   bigint NOT NULL,
    duration_max   bigint NOT NULL,
    durations      bytea NOT NULL,
    request_bytes  bigint NOT NULL,
    response_bytes bigint NOT NULL,
    response_sizes bytea NOT NULL,
    PRIMARY KEY (property_id, day, dim, value, route, method, status_class),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

-- A row per property and local hour that has rows: built_at once the hour is folded, dirty when rows
-- arrived after that, capped when the hour reached its dimension rows, day_built_at once the hour's
-- local day was folded from it (cleared when the hour is folded again).
CREATE TABLE monitor_rollup_hours (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    hour         timestamptz NOT NULL,
    day          date NOT NULL,
    built_at     timestamptz,
    dirty        boolean NOT NULL DEFAULT false,
    capped       boolean NOT NULL DEFAULT false,
    day_built_at timestamptz,
    PRIMARY KEY (property_id, hour),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX monitor_rollup_hours_todo_idx ON monitor_rollup_hours (workspace_id, property_id, hour)
    WHERE built_at IS NULL OR dirty OR day_built_at IS NULL;
CREATE INDEX monitor_rollup_hours_day_idx ON monitor_rollup_hours (property_id, day);

CREATE TABLE monitor_ingest_stats (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    day          date NOT NULL,
    reason       text NOT NULL CHECK (reason IN ('accepted', 'invalid', 'late', 'rate', 'over_limit', 'buffer')),
    records      bigint NOT NULL,
    PRIMARY KEY (property_id, day, reason),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

-- Request records counted against YUVA_MONITOR_MONTHLY_REQUESTS, apart from analytics events.
ALTER TABLE usage_counters ADD COLUMN monitor_requests bigint NOT NULL DEFAULT 0;

-- The operator's override of YUVA_MONITOR_MONTHLY_REQUESTS; 0 is no limit.
ALTER TABLE workspaces ADD COLUMN monitor_monthly_requests bigint CHECK (monitor_monthly_requests >= 0);

-- +goose Down
ALTER TABLE workspaces DROP COLUMN monitor_monthly_requests;
ALTER TABLE usage_counters DROP COLUMN monitor_requests;
DROP TABLE monitor_ingest_stats;
DROP TABLE monitor_rollup_hours;
DROP TABLE monitor_dim_day;
DROP TABLE monitor_day;
DROP TABLE monitor_dim_hour;
DROP TABLE monitor_hour;
DROP TABLE monitor_minute;
DROP TABLE monitor_names;
DROP TABLE monitor_routes;
DELETE FROM analytics_properties WHERE kind = 'api';
ALTER TABLE analytics_properties
    DROP CONSTRAINT analytics_properties_api_check,
    DROP CONSTRAINT analytics_properties_platforms_check,
    ADD CONSTRAINT analytics_properties_platforms_check CHECK (cardinality(platforms) >= 1 AND platforms <@ ARRAY['web', 'ios', 'android']),
    DROP CONSTRAINT analytics_properties_domains_check,
    ADD CONSTRAINT analytics_properties_domains_check CHECK (cardinality(domains) BETWEEN 1 AND 50),
    DROP COLUMN monitor_hour_days,
    DROP COLUMN apdex_ms,
    DROP COLUMN kind;
