-- +goose Up
-- A website or app the workspace measures. public_key names it on the collection endpoint and is
-- unique across the server; previous_key keeps working until previous_key_until after a rotation.
-- visitor_key is sealed under the master key and used only for consented visitors.
CREATE TABLE analytics_properties (
    id                 uuid PRIMARY KEY,
    workspace_id       uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    inbox_id           uuid,
    name               text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    public_key         text NOT NULL UNIQUE,
    previous_key       text UNIQUE,
    previous_key_until timestamptz,
    domains            text[] NOT NULL CHECK (cardinality(domains) BETWEEN 1 AND 50),
    platforms          text[] NOT NULL DEFAULT '{web}' CHECK (cardinality(platforms) >= 1 AND platforms <@ ARRAY['web', 'ios', 'android']),
    timezone           text NOT NULL DEFAULT 'UTC',
    currency           char(3) NOT NULL DEFAULT 'EUR',
    collection         text NOT NULL DEFAULT 'anonymous' CHECK (collection IN ('anonymous', 'consent')),
    consent_features   text[] NOT NULL DEFAULT '{}' CHECK (consent_features <@ ARRAY['persistent', 'identify']),
    audience           text NOT NULL DEFAULT 'general' CHECK (audience IN ('general', 'children')),
    excluded_paths     text[] NOT NULL DEFAULT '{}' CHECK (cardinality(excluded_paths) <= 50),
    excluded_ips       cidr[] NOT NULL DEFAULT '{}' CHECK (cardinality(excluded_ips) <= 50),
    path_rules         jsonb NOT NULL DEFAULT '[]',
    tracked_props      text[] NOT NULL DEFAULT '{}' CHECK (cardinality(tracked_props) <= 10),
    retention_months   integer DEFAULT 25 CHECK (retention_months BETWEEN 1 AND 120),
    visitor_key        bytea NOT NULL,
    rollup_version     integer NOT NULL DEFAULT 0,
    first_event_at     timestamptz,
    last_event_at      timestamptz,
    created_by         uuid,
    created_at         timestamptz NOT NULL,
    updated_at         timestamptz NOT NULL,
    deleted_at         timestamptz,
    UNIQUE (workspace_id, id),
    CHECK (audience <> 'children' OR (consent_features = '{}' AND tracked_props = '{}')),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE SET NULL (inbox_id)
);
CREATE INDEX analytics_properties_workspace_idx ON analytics_properties (workspace_id, created_at) WHERE deleted_at IS NULL;
CREATE INDEX analytics_properties_inbox_idx ON analytics_properties (workspace_id, inbox_id) WHERE deleted_at IS NULL;

-- One random salt per property and local day, deleted once expires_at (the day's end plus the
-- grace) has passed. Never derived from anything that persists.
CREATE TABLE analytics_salts (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    day          date NOT NULL,
    salt         bytea NOT NULL CHECK (length(salt) = 32),
    expires_at   timestamptz NOT NULL,
    PRIMARY KEY (property_id, day),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX analytics_salts_expiry_idx ON analytics_salts (workspace_id, expires_at);

-- Raw rows, partitioned by the property's local day. No foreign keys: a property's rows are
-- deleted explicitly, per partition, before the property row.
CREATE TABLE analytics_visits (
    workspace_id  uuid NOT NULL,
    property_id   uuid NOT NULL,
    day           date NOT NULL,
    visit         bigint NOT NULL,
    visitor       bigint NOT NULL,
    started_at    timestamptz NOT NULL,
    ended_at      timestamptz NOT NULL,
    hostname      text NOT NULL,
    entry_path    text NOT NULL,
    exit_path     text NOT NULL,
    views         integer NOT NULL DEFAULT 0,
    events        integer NOT NULL DEFAULT 0,
    interactive   boolean NOT NULL DEFAULT false,
    engaged_ms    bigint NOT NULL DEFAULT 0,
    channel       smallint NOT NULL,
    source        text NOT NULL DEFAULT '',
    referrer_host text NOT NULL DEFAULT '',
    referrer_path text NOT NULL DEFAULT '',
    utm_source    text NOT NULL DEFAULT '',
    utm_medium    text NOT NULL DEFAULT '',
    utm_campaign  text NOT NULL DEFAULT '',
    utm_term      text NOT NULL DEFAULT '',
    utm_content   text NOT NULL DEFAULT '',
    country       text NOT NULL DEFAULT '',
    region        text NOT NULL DEFAULT '',
    city          text NOT NULL DEFAULT '',
    device        smallint NOT NULL DEFAULT 0,
    browser       text NOT NULL DEFAULT '',
    browser_major smallint NOT NULL DEFAULT 0,
    os            text NOT NULL DEFAULT '',
    os_major      smallint NOT NULL DEFAULT 0,
    screen        smallint NOT NULL DEFAULT 0,
    language      text NOT NULL DEFAULT '',
    platform      smallint NOT NULL,
    app_version   text NOT NULL DEFAULT '',
    persistent    boolean NOT NULL DEFAULT false,
    contact_id    uuid,
    bot           boolean NOT NULL DEFAULT false,
    PRIMARY KEY (property_id, day, visit)
) PARTITION BY RANGE (day);
CREATE INDEX analytics_visits_visitor_idx ON analytics_visits (property_id, day, visitor);
CREATE INDEX analytics_visits_contact_idx ON analytics_visits (workspace_id, contact_id) WHERE contact_id IS NOT NULL;

CREATE TABLE analytics_events (
    workspace_id  uuid NOT NULL,
    property_id   uuid NOT NULL,
    day           date NOT NULL,
    ts            timestamptz NOT NULL,
    visit         bigint NOT NULL,
    visitor       bigint NOT NULL,
    kind          smallint NOT NULL,
    name          text NOT NULL DEFAULT '',
    hostname      text NOT NULL,
    path          text NOT NULL,
    props         jsonb,
    revenue_minor bigint,
    currency      text,
    engaged_ms    integer,
    scroll        smallint,
    interactive   boolean NOT NULL DEFAULT true
) PARTITION BY RANGE (day);
CREATE INDEX analytics_events_time_idx ON analytics_events (property_id, ts);
CREATE INDEX analytics_events_visit_idx ON analytics_events (property_id, visit, ts);

CREATE TABLE analytics_daily (
    workspace_id  uuid NOT NULL,
    property_id   uuid NOT NULL,
    day           date NOT NULL,
    dim           smallint NOT NULL,
    value         text NOT NULL,
    visitors      bigint NOT NULL,
    visits        bigint NOT NULL,
    views         bigint NOT NULL,
    bounces       bigint NOT NULL,
    duration_s    bigint NOT NULL,
    events        bigint NOT NULL,
    engaged_ms    bigint NOT NULL,
    engaged_views bigint NOT NULL,
    scroll_sum    bigint NOT NULL,
    scroll_views  bigint NOT NULL,
    PRIMARY KEY (property_id, dim, day, value),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE analytics_hourly (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    day          date NOT NULL,
    hour         smallint NOT NULL CHECK (hour BETWEEN 0 AND 23),
    visitors     bigint NOT NULL,
    visits       bigint NOT NULL,
    views        bigint NOT NULL,
    events       bigint NOT NULL,
    PRIMARY KEY (property_id, day, hour),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

-- A row per property and day that has raw rows: built_at once the day is rolled up, dirty when
-- rows arrived or were deleted after that.
CREATE TABLE analytics_rollup_days (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    day          date NOT NULL,
    built_at     timestamptz,
    dirty        boolean NOT NULL DEFAULT false,
    PRIMARY KEY (property_id, day),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX analytics_rollup_days_todo_idx ON analytics_rollup_days (workspace_id, property_id) WHERE built_at IS NULL OR dirty;

CREATE TABLE analytics_ingest_stats (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    day          date NOT NULL,
    reason       text NOT NULL CHECK (reason IN ('accepted', 'bot', 'rate', 'origin', 'invalid', 'late', 'over_limit', 'buffer', 'excluded', 'scrubbed')),
    events       bigint NOT NULL,
    PRIMARY KEY (property_id, day, reason),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

-- analytics_limit_reached_at: when the workspace hit its monthly event cap, so its owners and the
-- operator hear about it once a month.
ALTER TABLE usage_counters
    ADD COLUMN analytics_events bigint NOT NULL DEFAULT 0,
    ADD COLUMN analytics_limit_reached_at timestamptz;

-- The operator's override of YUVA_ANALYTICS_MONTHLY_EVENTS; 0 is no limit.
ALTER TABLE workspaces ADD COLUMN analytics_monthly_events bigint CHECK (analytics_monthly_events >= 0);

ALTER TABLE operator_notices
    DROP CONSTRAINT operator_notices_kind_check,
    ADD CONSTRAINT operator_notices_kind_check
        CHECK (kind IN ('signup', 'workspace', 'broadcast_approval', 'broadcast_suspended', 'broadcast_paused', 'analytics_limit'));

-- +goose Down
DELETE FROM operator_notices WHERE kind = 'analytics_limit';
ALTER TABLE operator_notices
    DROP CONSTRAINT operator_notices_kind_check,
    ADD CONSTRAINT operator_notices_kind_check
        CHECK (kind IN ('signup', 'workspace', 'broadcast_approval', 'broadcast_suspended', 'broadcast_paused'));
ALTER TABLE workspaces DROP COLUMN analytics_monthly_events;
ALTER TABLE usage_counters DROP COLUMN analytics_limit_reached_at, DROP COLUMN analytics_events;
DROP TABLE analytics_ingest_stats;
DROP TABLE analytics_rollup_days;
DROP TABLE analytics_hourly;
DROP TABLE analytics_daily;
DROP TABLE analytics_events;
DROP TABLE analytics_visits;
DROP TABLE analytics_salts;
DROP TABLE analytics_properties;
