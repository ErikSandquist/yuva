-- +goose Up
-- token_hash is unique across the server: the share page finds its link by it before the
-- workspace is known. password_hash is an argon2id PHC string.
CREATE TABLE analytics_shares (
    id             uuid PRIMARY KEY,
    workspace_id   uuid NOT NULL,
    property_id    uuid NOT NULL,
    name           text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    token_hash     bytea NOT NULL UNIQUE,
    token_prefix   text NOT NULL,
    views          text[] NOT NULL CHECK (cardinality(views) BETWEEN 1 AND 8),
    password_hash  text,
    embed_origins  text[] NOT NULL DEFAULT '{}',
    expires_at     timestamptz,
    revoked_at     timestamptz,
    last_viewed_at timestamptz,
    created_by     uuid,
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, created_by) REFERENCES members (workspace_id, id) ON DELETE SET NULL (created_by)
);
CREATE INDEX analytics_shares_property_idx ON analytics_shares (workspace_id, property_id, created_at);

-- params holds the request (a view's query, a daily export's dimension); storage_key the file once
-- it is written. Files are deleted with their row when it expires, its property or its workspace
-- goes.
CREATE TABLE analytics_exports (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    kind         text NOT NULL CHECK (kind IN ('view', 'daily', 'raw')),
    format       text NOT NULL CHECK (format IN ('csv', 'ndjson')),
    status       text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'done', 'failed')),
    params       jsonb NOT NULL DEFAULT '{}',
    from_day     date NOT NULL,
    to_day       date NOT NULL,
    progress     smallint NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    rows         bigint NOT NULL DEFAULT 0,
    size_bytes   bigint NOT NULL DEFAULT 0,
    storage_key  text,
    error        text,
    created_by   uuid,
    created_at   timestamptz NOT NULL,
    finished_at  timestamptz,
    expires_at   timestamptz NOT NULL,
    CHECK (kind <> 'raw' OR format = 'ndjson'),
    FOREIGN KEY (workspace_id, property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, created_by) REFERENCES members (workspace_id, id) ON DELETE SET NULL (created_by)
);
CREATE INDEX analytics_exports_property_idx ON analytics_exports (workspace_id, property_id, created_at);
CREATE INDEX analytics_exports_expires_idx ON analytics_exports (workspace_id, expires_at);

-- +goose Down
DROP TABLE analytics_exports;
DROP TABLE analytics_shares;
