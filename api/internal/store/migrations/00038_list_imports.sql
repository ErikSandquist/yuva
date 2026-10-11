-- +goose Up
CREATE TABLE list_imports (
    id             uuid PRIMARY KEY,
    workspace_id   uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    list_id        uuid NOT NULL,
    member_id      uuid,
    api_key_id     uuid,
    state          text NOT NULL CHECK (state IN ('validating', 'ready', 'importing', 'done', 'failed', 'cancelled')),
    filename       text NOT NULL CHECK (length(filename) BETWEEN 1 AND 255),
    storage_key    text UNIQUE,
    encoding       text,
    delimiter      text,
    columns        jsonb NOT NULL DEFAULT '[]',
    preview        jsonb,
    domains        jsonb NOT NULL DEFAULT '{}',
    attestation    text NOT NULL CHECK (length(attestation) BETWEEN 1 AND 2000),
    consent_source text NOT NULL CHECK (length(consent_source) BETWEEN 1 AND 1000),
    reconfirm      boolean NOT NULL DEFAULT false,
    rows_total     integer NOT NULL DEFAULT 0,
    rows_done      integer NOT NULL DEFAULT 0,
    counts         jsonb NOT NULL DEFAULT '{}',
    error          text CHECK (length(error) <= 2000),
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL,
    started_at     timestamptz,
    finished_at    timestamptz,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, list_id) REFERENCES mailing_lists (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, member_id) REFERENCES members (workspace_id, id) ON DELETE SET NULL (member_id)
);
CREATE INDEX list_imports_list_idx ON list_imports (workspace_id, list_id, created_at DESC, id DESC);
CREATE UNIQUE INDEX list_imports_running_idx ON list_imports (workspace_id) WHERE state IN ('validating', 'importing');

-- Refused rows of an import, written in the transaction of their batch so a resumed job never
-- repeats one; the error CSV is read from here and the rows go 7 days after the import ends.
CREATE TABLE list_import_errors (
    workspace_id uuid NOT NULL,
    import_id    uuid NOT NULL,
    row          integer NOT NULL,
    email        text NOT NULL CHECK (length(email) <= 320),
    reason       text NOT NULL CHECK (reason IN ('invalid_email', 'no_mail_domain', 'role_address', 'duplicate', 'suppressed', 'opted_out', 'unsubscribed')),
    PRIMARY KEY (workspace_id, import_id, row),
    FOREIGN KEY (workspace_id, import_id) REFERENCES list_imports (workspace_id, id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE list_import_errors;
DROP TABLE list_imports;
