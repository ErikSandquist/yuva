-- +goose Up
CREATE TABLE status_page_slugs (
    slug         text PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    page_id      uuid NOT NULL,
    created_at   timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, page_id) REFERENCES status_pages (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX status_page_slugs_page_idx ON status_page_slugs (workspace_id, page_id);

-- +goose Down
DROP TABLE status_page_slugs;
