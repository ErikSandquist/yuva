-- +goose Up
-- Folds of open days: the numbers of the visitors whose every visit begun by until_at had ended
-- half an hour before it, so a query adds only the other visitors' raw rows. A fold counts while
-- the day's late_seq (and, for engagement metrics, engagement_seq) still has the value it read;
-- they move when rows a fold may hold change afterwards.
ALTER TABLE analytics_rollup_days
    ADD COLUMN late_seq bigint NOT NULL DEFAULT 0,
    ADD COLUMN engagement_seq bigint NOT NULL DEFAULT 0;

CREATE TABLE analytics_partial_days (
    workspace_id   uuid NOT NULL,
    property_id    uuid NOT NULL,
    day            date NOT NULL,
    until_at       timestamptz NOT NULL,
    late_seq       bigint NOT NULL,
    engagement_seq bigint NOT NULL,
    built_at       timestamptz NOT NULL,
    PRIMARY KEY (property_id, day),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE analytics_partial_daily (
    workspace_id  uuid NOT NULL,
    property_id   uuid NOT NULL,
    day           date NOT NULL,
    dim           smallint NOT NULL,
    value         text COLLATE "C" NOT NULL,
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

CREATE TABLE analytics_partial_hourly (
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

-- +goose Down
DROP TABLE analytics_partial_hourly;
DROP TABLE analytics_partial_daily;
DROP TABLE analytics_partial_days;
ALTER TABLE analytics_rollup_days DROP COLUMN engagement_seq, DROP COLUMN late_seq;
