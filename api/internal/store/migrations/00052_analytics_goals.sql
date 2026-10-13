-- +goose Up
-- goals_version moves whenever a goal's definition or the tracked props change. A rolled-up day
-- records the version its goal and prop rows were built with: a goal counts from rollups on days
-- built at or after its own version, and days of the raw window built before the property's
-- version are rolled up again.
ALTER TABLE analytics_properties ADD COLUMN goals_version bigint NOT NULL DEFAULT 0;
ALTER TABLE analytics_rollup_days ADD COLUMN goals_version bigint NOT NULL DEFAULT 0;

CREATE TABLE analytics_goals (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    kind         text NOT NULL CHECK (kind IN ('pageview', 'event', 'scroll')),
    path_pattern text,
    event_name   text,
    prop_key     text,
    prop_value   text,
    scroll_pct   smallint CHECK (scroll_pct BETWEEN 1 AND 100),
    version      bigint NOT NULL,
    counts_from  date NOT NULL,
    created_by   uuid,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (property_id, name),
    CHECK ((kind IN ('pageview', 'scroll')) = (path_pattern IS NOT NULL)),
    CHECK ((kind = 'event') = (event_name IS NOT NULL)),
    CHECK ((kind = 'scroll') = (scroll_pct IS NOT NULL)),
    CHECK ((prop_key IS NULL) = (prop_value IS NULL) AND (prop_key IS NULL OR kind = 'event')),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, created_by) REFERENCES members (workspace_id, id) ON DELETE SET NULL (created_by)
);
CREATE INDEX analytics_goals_property_idx ON analytics_goals (workspace_id, property_id, created_at);

-- steps are goal ids of the same property, checked by the handler; a goal in a funnel cannot be
-- deleted.
CREATE TABLE analytics_funnels (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    steps        uuid[] NOT NULL CHECK (cardinality(steps) BETWEEN 2 AND 8),
    created_by   uuid,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    UNIQUE (property_id, name),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, created_by) REFERENCES members (workspace_id, id) ON DELETE SET NULL (created_by)
);
CREATE INDEX analytics_funnels_property_idx ON analytics_funnels (workspace_id, property_id, created_at);

CREATE TABLE analytics_segments (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    filters      jsonb NOT NULL,
    created_by   uuid,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    UNIQUE (property_id, name),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, created_by) REFERENCES members (workspace_id, id) ON DELETE SET NULL (created_by)
);
CREATE INDEX analytics_segments_property_idx ON analytics_segments (workspace_id, property_id, created_at);

-- kind note is a member's; the others Yuva writes itself and nobody edits.
CREATE TABLE analytics_annotations (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    kind         text NOT NULL CHECK (kind IN ('note', 'key_rotated', 'goal_created')),
    starts_on    date NOT NULL,
    ends_on      date NOT NULL CHECK (ends_on >= starts_on),
    text         text NOT NULL CHECK (length(text) <= 300),
    goal_id      uuid,
    created_by   uuid,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    CHECK (kind <> 'note' OR length(text) >= 1),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, goal_id) REFERENCES analytics_goals (workspace_id, id) ON DELETE SET NULL (goal_id),
    FOREIGN KEY (workspace_id, created_by) REFERENCES members (workspace_id, id) ON DELETE SET NULL (created_by)
);
CREATE INDEX analytics_annotations_property_idx ON analytics_annotations (workspace_id, property_id, starts_on);

-- Per day, goal, dimension and value: rows with an empty currency hold the converting visitors,
-- their visits and the completions; a row per currency holds the revenue of the completions.
CREATE TABLE analytics_daily_goals (
    workspace_id   uuid NOT NULL,
    property_id    uuid NOT NULL,
    day            date NOT NULL,
    goal_id        uuid NOT NULL,
    dim            smallint NOT NULL,
    value          text COLLATE "C" NOT NULL,
    currency       text COLLATE "C" NOT NULL DEFAULT '',
    visitors       bigint NOT NULL,
    visits         bigint NOT NULL,
    completions    bigint NOT NULL,
    revenue_minor  bigint NOT NULL DEFAULT 0,
    revenue_events bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (property_id, goal_id, dim, day, value, currency),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, goal_id) REFERENCES analytics_goals (workspace_id, id) ON DELETE CASCADE
);

-- Per day, custom event name ('' for all of them), tracked prop ('' for the event itself) and value:
-- like analytics_daily_goals, the empty currency holds the counts and each currency its revenue.
CREATE TABLE analytics_daily_props (
    workspace_id   uuid NOT NULL,
    property_id    uuid NOT NULL,
    day            date NOT NULL,
    name           text COLLATE "C" NOT NULL,
    prop           text COLLATE "C" NOT NULL,
    value          text COLLATE "C" NOT NULL,
    currency       text COLLATE "C" NOT NULL DEFAULT '',
    visitors       bigint NOT NULL,
    visits         bigint NOT NULL,
    events         bigint NOT NULL,
    revenue_minor  bigint NOT NULL DEFAULT 0,
    revenue_events bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (property_id, name, prop, day, value, currency),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

-- Custom events are few next to page views; this index serves event goals, event breakdowns and
-- funnels without growing the cost of writing page views.
CREATE INDEX analytics_events_custom_idx ON analytics_events (property_id, name, ts) WHERE kind = 3;

-- +goose Down
DROP INDEX analytics_events_custom_idx;
DROP TABLE analytics_daily_props;
DROP TABLE analytics_daily_goals;
DROP TABLE analytics_annotations;
DROP TABLE analytics_segments;
DROP TABLE analytics_funnels;
DROP TABLE analytics_goals;
ALTER TABLE analytics_rollup_days DROP COLUMN goals_version;
ALTER TABLE analytics_properties DROP COLUMN goals_version;
