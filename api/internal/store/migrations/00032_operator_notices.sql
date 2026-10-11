-- +goose Up
CREATE TABLE operator_notices (
    id             uuid PRIMARY KEY,
    kind           text NOT NULL CHECK (kind IN ('signup', 'workspace')),
    email          text NOT NULL,
    reader         boolean NOT NULL DEFAULT false,
    workspace_name text,
    ip             text NOT NULL DEFAULT '',
    country        text,
    created_at     timestamptz NOT NULL,
    sent_at        timestamptz
);
CREATE INDEX operator_notices_unsent_idx ON operator_notices (created_at) WHERE sent_at IS NULL;
CREATE INDEX operator_notices_sent_idx ON operator_notices (sent_at) WHERE sent_at IS NOT NULL;

-- +goose Down
DROP TABLE operator_notices;
