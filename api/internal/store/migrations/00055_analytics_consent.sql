-- +goose Up
-- Consent features. A visitor who agreed to `persistent` sends a random id their device keeps for
-- at most 13 months; its hash under the property's visitor_key stands for them on every day, so
-- their visits are persistent and visitor_type says whether that day was their first (1) or a
-- later one (2). Every other visit keeps visitor_type 0 and persistent false.
ALTER TABLE analytics_visits ADD COLUMN visitor_type smallint NOT NULL DEFAULT 0;
CREATE INDEX analytics_visits_persistent_idx ON analytics_visits (property_id, visitor) WHERE persistent;

-- A consented visitor of a property: the local days they were first and last seen, and the contact
-- an identity token linked them to. Deleted 13 months after first_day by the cohorts job, when the
-- visitor withdraws, when a member deletes them, and with their contact.
CREATE TABLE analytics_persistent_visitors (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    visitor      bigint NOT NULL,
    first_day    date NOT NULL,
    last_day     date NOT NULL,
    contact_id   uuid,
    linked_at    timestamptz,
    PRIMARY KEY (property_id, visitor),
    UNIQUE (workspace_id, property_id, visitor),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, contact_id) REFERENCES contacts (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX analytics_persistent_visitors_first_idx ON analytics_persistent_visitors (workspace_id, first_day);
CREATE INDEX analytics_persistent_visitors_contact_idx ON analytics_persistent_visitors (workspace_id, contact_id) WHERE contact_id IS NOT NULL;

-- The local days a consented visitor had a visit that was not a bot's: their cohort days.
CREATE TABLE analytics_persistent_days (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    visitor      bigint NOT NULL,
    day          date NOT NULL,
    PRIMARY KEY (property_id, visitor, day),
    FOREIGN KEY (workspace_id, property_id, visitor) REFERENCES analytics_persistent_visitors (workspace_id, property_id, visitor) ON DELETE CASCADE
);
CREATE INDEX analytics_persistent_days_day_idx ON analytics_persistent_days (property_id, day);

-- Retention, rebuilt by the cohorts job: per grain, the cohort (the start of the bucket holding the
-- visitors' first_day), the period since it and the distinct visitors of the cohort seen in it.
CREATE TABLE analytics_cohorts (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    grain        text NOT NULL CHECK (grain IN ('day', 'week', 'month')),
    cohort       date NOT NULL,
    period       integer NOT NULL CHECK (period >= 0),
    visitors     bigint NOT NULL,
    PRIMARY KEY (property_id, grain, cohort, period),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

ALTER TABLE analytics_properties ADD COLUMN cohorts_built_at timestamptz;

-- +goose Down
ALTER TABLE analytics_properties DROP COLUMN cohorts_built_at;
DROP TABLE analytics_cohorts;
DROP TABLE analytics_persistent_days;
DROP TABLE analytics_persistent_visitors;
DROP INDEX analytics_visits_persistent_idx;
ALTER TABLE analytics_visits DROP COLUMN visitor_type;
