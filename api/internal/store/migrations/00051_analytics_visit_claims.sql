-- +goose Up
-- The visit a process started for a visitor of a property's day, until its rows have reached
-- analytics_visits. Another process that would start a visit for the same visitor within the claim
-- window joins this one instead, so one visit is never split between processes before a flush.
-- Rows older than the window no longer matter and are deleted with the expired salts.
CREATE TABLE analytics_visit_claims (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    day          date NOT NULL,
    visitor      bigint NOT NULL,
    visit        bigint NOT NULL,
    claimed_at   timestamptz NOT NULL,
    PRIMARY KEY (property_id, day, visitor),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX analytics_visit_claims_age_idx ON analytics_visit_claims (workspace_id, claimed_at);

-- +goose Down
DROP TABLE analytics_visit_claims;
