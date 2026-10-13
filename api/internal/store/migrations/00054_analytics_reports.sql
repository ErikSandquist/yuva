-- +goose Up
-- The support rollup: what a property's linked inbox received from each page (host and path, after
-- the property's path rules) per local day. A day is read from it only while its row in
-- analytics_support_days names the property's current inbox; otherwise from the inbox's own tables.
CREATE TABLE analytics_support_days (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    day          date NOT NULL,
    inbox_id     uuid NOT NULL,
    built_at     timestamptz NOT NULL,
    PRIMARY KEY (property_id, day),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE analytics_daily_support (
    workspace_id  uuid NOT NULL,
    property_id   uuid NOT NULL,
    day           date NOT NULL,
    host          text COLLATE "C" NOT NULL,
    path          text COLLATE "C" NOT NULL,
    conversations bigint NOT NULL DEFAULT 0,
    feedback      bigint NOT NULL DEFAULT 0,
    ratings_up    bigint NOT NULL DEFAULT 0,
    ratings_down  bigint NOT NULL DEFAULT 0,
    discussions   bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (property_id, day, host, path),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX conversations_page_created_idx ON conversations (workspace_id, inbox_id, created_at) WHERE page_url IS NOT NULL;

-- last_period is the first day of the week or month the last scheduled report covered, so each
-- period is mailed once even when several processes run the job.
CREATE TABLE analytics_subscriptions (
    workspace_id   uuid NOT NULL,
    property_id    uuid NOT NULL,
    member_id      uuid NOT NULL,
    report         text NOT NULL DEFAULT 'none' CHECK (report IN ('none', 'weekly', 'monthly')),
    alerts         text[] NOT NULL DEFAULT '{}',
    last_period    date,
    last_report_at timestamptz,
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL,
    PRIMARY KEY (property_id, member_id),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, member_id) REFERENCES members (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX analytics_subscriptions_member_idx ON analytics_subscriptions (workspace_id, member_id);

-- key makes each alert fire once: the day for most kinds, the day and goal for a goal drop, the
-- referring host for a new referrer.
CREATE TABLE analytics_alerts (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL,
    property_id   uuid NOT NULL,
    kind          text NOT NULL CHECK (kind IN ('spike', 'drop', 'goal_drop', 'support_load', 'new_referrer')),
    key           text NOT NULL,
    day           date NOT NULL,
    hours_from    timestamptz,
    hours_to      timestamptz,
    metric        text NOT NULL,
    current       double precision NOT NULL,
    baseline      double precision NOT NULL,
    goal_id       uuid,
    goal_name     text,
    referrer      text,
    text          text NOT NULL,
    annotation_id uuid,
    created_at    timestamptz NOT NULL,
    UNIQUE (property_id, kind, key),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, goal_id) REFERENCES analytics_goals (workspace_id, id) ON DELETE SET NULL (goal_id)
);
CREATE INDEX analytics_alerts_property_idx ON analytics_alerts (workspace_id, property_id, created_at DESC);

ALTER TABLE analytics_annotations
    DROP CONSTRAINT analytics_annotations_kind_check,
    ADD CONSTRAINT analytics_annotations_kind_check CHECK (kind IN ('note', 'key_rotated', 'goal_created', 'alert')),
    ADD COLUMN alert_id uuid;

-- Broadcasts already past their draft keep their links as they were sent.
ALTER TABLE broadcasts
    ADD COLUMN utm boolean NOT NULL DEFAULT true,
    ADD COLUMN utm_campaign text NOT NULL DEFAULT '' CHECK (length(utm_campaign) <= 100);
UPDATE broadcasts SET utm = (state = 'draft'),
    utm_campaign = coalesce(nullif(left(trim(BOTH '-' FROM regexp_replace(lower(translate(name, 'ÇĞİIÖŞÜçğıöşüÄäß', 'cgiiosucgiosuaas')), '[^a-z0-9_.]+', '-', 'g')), 100), ''),
        'broadcast-' || left(id::text, 8));

-- +goose Down
ALTER TABLE broadcasts DROP COLUMN utm_campaign, DROP COLUMN utm;
DELETE FROM analytics_annotations WHERE kind = 'alert';
ALTER TABLE analytics_annotations
    DROP COLUMN alert_id,
    DROP CONSTRAINT analytics_annotations_kind_check,
    ADD CONSTRAINT analytics_annotations_kind_check CHECK (kind IN ('note', 'key_rotated', 'goal_created'));
DROP TABLE analytics_alerts;
DROP TABLE analytics_subscriptions;
DROP INDEX conversations_page_created_idx;
DROP TABLE analytics_daily_support;
DROP TABLE analytics_support_days;
