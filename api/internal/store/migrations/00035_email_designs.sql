-- +goose Up
CREATE TABLE email_designs (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    inbox_id      uuid,
    name          text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    document      jsonb NOT NULL,
    version       integer NOT NULL DEFAULT 1 CHECK (version >= 1),
    template_slug text CHECK (template_slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    created_by    uuid,
    updated_by    uuid,
    archived_at   timestamptz,
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE SET NULL (inbox_id),
    FOREIGN KEY (workspace_id, created_by) REFERENCES members (workspace_id, id) ON DELETE SET NULL (created_by),
    FOREIGN KEY (workspace_id, updated_by) REFERENCES members (workspace_id, id) ON DELETE SET NULL (updated_by)
);
CREATE INDEX email_designs_list_idx ON email_designs (workspace_id, updated_at DESC, id DESC) WHERE archived_at IS NULL;
CREATE INDEX email_designs_archived_idx ON email_designs (workspace_id, updated_at DESC, id DESC) WHERE archived_at IS NOT NULL;

-- Snapshots: one at most every 10 minutes of editing, on request, on restore, on test send and on
-- scheduling; the newest 50 of a design are kept.
CREATE TABLE email_design_versions (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    design_id    uuid NOT NULL,
    version      integer NOT NULL,
    document     jsonb NOT NULL,
    reason       text NOT NULL CHECK (reason IN ('autosave', 'test', 'scheduled', 'restored', 'manual')),
    member_id    uuid,
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, design_id, version),
    FOREIGN KEY (workspace_id, design_id) REFERENCES email_designs (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, member_id) REFERENCES members (workspace_id, id) ON DELETE SET NULL (member_id)
);

-- The token is looked up server-wide by /i/{token} before the workspace is known (architecture.md,
-- Hosting for others later).
CREATE TABLE email_images (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    token        text NOT NULL CHECK (token ~ '^[a-z2-7]{26}$'),
    storage_key  text NOT NULL UNIQUE,
    content_type text NOT NULL CHECK (content_type IN ('image/png', 'image/jpeg', 'image/gif')),
    width        integer NOT NULL CHECK (width BETWEEN 1 AND 1200),
    height       integer NOT NULL CHECK (height BETWEEN 1 AND 4000),
    size_bytes   bigint NOT NULL CHECK (size_bytes > 0),
    sha256       bytea NOT NULL,
    filename     text NOT NULL CHECK (length(filename) BETWEEN 1 AND 255),
    member_id    uuid,
    sent_at      timestamptz,
    created_at   timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, sha256),
    FOREIGN KEY (workspace_id, member_id) REFERENCES members (workspace_id, id) ON DELETE SET NULL (member_id)
);
CREATE UNIQUE INDEX email_images_token_key ON email_images (token);
CREATE INDEX email_images_list_idx ON email_images (workspace_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE email_images;
DROP TABLE email_design_versions;
DROP TABLE email_designs;
