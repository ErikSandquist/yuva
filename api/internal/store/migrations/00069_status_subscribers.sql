-- +goose Up
ALTER TABLE mailing_lists ADD COLUMN kind text NOT NULL DEFAULT 'list' CHECK (kind IN ('list', 'status'));
ALTER TABLE mailing_lists ADD CONSTRAINT mailing_lists_status_fixed
    CHECK (kind <> 'status' OR (NOT public AND NOT commercial AND double_opt_in AND NOT welcome));

ALTER TABLE status_pages ADD COLUMN mailing_list_id uuid;
ALTER TABLE status_pages ADD COLUMN email_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE status_pages ADD CONSTRAINT status_pages_mailing_list_id_key UNIQUE (mailing_list_id);
ALTER TABLE status_pages ADD CONSTRAINT status_pages_mailing_list_fkey
    FOREIGN KEY (workspace_id, mailing_list_id) REFERENCES mailing_lists (workspace_id, id) ON DELETE SET NULL (mailing_list_id);

-- A status subscription without rows gets the notices of every component.
CREATE TABLE status_subscriber_components (
    workspace_id    uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    subscription_id uuid NOT NULL,
    component_id    uuid NOT NULL,
    PRIMARY KEY (subscription_id, component_id),
    FOREIGN KEY (workspace_id, subscription_id) REFERENCES list_subscriptions (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, component_id) REFERENCES status_components (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX status_subscriber_components_component_idx ON status_subscriber_components (workspace_id, component_id);

-- +goose Down
DROP TABLE status_subscriber_components;
ALTER TABLE status_pages DROP CONSTRAINT status_pages_mailing_list_fkey;
ALTER TABLE status_pages DROP CONSTRAINT status_pages_mailing_list_id_key;
ALTER TABLE status_pages DROP COLUMN email_enabled;
ALTER TABLE status_pages DROP COLUMN mailing_list_id;
ALTER TABLE mailing_lists DROP CONSTRAINT mailing_lists_status_fixed;
ALTER TABLE mailing_lists DROP COLUMN kind;
