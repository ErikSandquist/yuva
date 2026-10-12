-- +goose Up
-- The plan joins threads here instead of in memory: every message id a scanned message has or
-- names is a node, and root is the node its thread is known by (union by size).
CREATE TABLE migration_plan_nodes (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    migration_id uuid NOT NULL,
    node         text NOT NULL CHECK (length(node) BETWEEN 1 AND 1000),
    root         text NOT NULL,
    size         integer NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_id, migration_id, node),
    FOREIGN KEY (workspace_id, migration_id) REFERENCES migrations (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX migration_plan_nodes_root_idx ON migration_plan_nodes (workspace_id, migration_id, root);

-- The name each address of the mailbox goes by most often, for the contacts the plan names.
CREATE TABLE migration_plan_names (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    migration_id uuid NOT NULL,
    email        text NOT NULL,
    name         text NOT NULL,
    PRIMARY KEY (workspace_id, migration_id, email),
    FOREIGN KEY (workspace_id, migration_id) REFERENCES migrations (workspace_id, id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE migration_plan_names;
DROP TABLE migration_plan_nodes;
