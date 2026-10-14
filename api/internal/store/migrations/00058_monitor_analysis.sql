-- +goose Up
-- Deploy markers (M16.3): the releases each service and environment of an API property brought, so
-- a release seen for the first time writes one deploy annotation, and a rolling deploy whose old
-- instances still report the release before does not write another.
CREATE TABLE monitor_releases (
    workspace_id  uuid NOT NULL,
    property_id   uuid NOT NULL,
    service       text COLLATE "C" NOT NULL,
    environment   text COLLATE "C" NOT NULL,
    release       text COLLATE "C" NOT NULL CHECK (length(release) BETWEEN 1 AND 80),
    first_seen_at timestamptz NOT NULL,
    PRIMARY KEY (property_id, service, environment, release),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE
);

-- A deploy annotation has its instant (at) for minute charts and says which release it marks.
ALTER TABLE analytics_annotations
    DROP CONSTRAINT analytics_annotations_kind_check,
    ADD CONSTRAINT analytics_annotations_kind_check CHECK (kind IN ('note', 'key_rotated', 'goal_created', 'alert', 'deploy')),
    ADD COLUMN at timestamptz,
    ADD COLUMN service text,
    ADD COLUMN environment text,
    ADD COLUMN release text,
    ADD CONSTRAINT analytics_annotations_deploy_check CHECK (kind <> 'deploy' OR (at IS NOT NULL AND release IS NOT NULL));

-- conversations per local day of an API property's linked inbox.
CREATE INDEX conversations_inbox_created_idx ON conversations (workspace_id, inbox_id, created_at);

-- +goose Down
DROP INDEX conversations_inbox_created_idx;
DELETE FROM analytics_annotations WHERE kind = 'deploy';
ALTER TABLE analytics_annotations
    DROP CONSTRAINT analytics_annotations_deploy_check,
    DROP COLUMN release,
    DROP COLUMN environment,
    DROP COLUMN service,
    DROP COLUMN at,
    DROP CONSTRAINT analytics_annotations_kind_check,
    ADD CONSTRAINT analytics_annotations_kind_check CHECK (kind IN ('note', 'key_rotated', 'goal_created', 'alert'));
DROP TABLE monitor_releases;
