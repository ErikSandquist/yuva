-- +goose Up
CREATE TABLE status_pages (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    inbox_id     uuid NOT NULL,
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    description  text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
    slug         text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(slug) BETWEEN 3 AND 48),
    logo_url     text CHECK (length(logo_url) <= 2000),
    accent_color text CHECK (accent_color ~ '^#[0-9a-fA-F]{6}$'),
    site_url     text CHECK (length(site_url) <= 2000),
    support_url  text CHECK (length(support_url) <= 2000),
    locale       text NOT NULL,
    timezone     text NOT NULL,
    history_days integer NOT NULL DEFAULT 90 CHECK (history_days BETWEEN 7 AND 90),
    show_uptime  boolean NOT NULL DEFAULT true,
    published    boolean NOT NULL DEFAULT false,
    -- changed_at moves with every change the public page shows, so cached answers can be checked.
    changed_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, inbox_id),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX status_pages_list_idx ON status_pages (workspace_id, created_at, id);

CREATE TABLE status_component_groups (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    page_id      uuid NOT NULL,
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    position     integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, page_id) REFERENCES status_pages (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX status_component_groups_page_idx ON status_component_groups (workspace_id, page_id);

CREATE TABLE status_components (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    page_id      uuid NOT NULL,
    group_id     uuid,
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    description  text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    position     integer NOT NULL DEFAULT 0,
    hidden       boolean NOT NULL DEFAULT false,
    state        text NOT NULL CHECK (state IN ('operational', 'degraded', 'partial_outage', 'major_outage', 'maintenance')),
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, page_id) REFERENCES status_pages (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, group_id) REFERENCES status_component_groups (workspace_id, id) ON DELETE SET NULL (group_id)
);
CREATE INDEX status_components_page_idx ON status_components (workspace_id, page_id);

CREATE TABLE status_component_states (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    component_id uuid NOT NULL,
    state        text NOT NULL CHECK (state IN ('operational', 'degraded', 'partial_outage', 'major_outage', 'maintenance')),
    source       text NOT NULL CHECK (source IN ('member', 'api_key', 'monitor', 'incident')),
    source_id    uuid,
    started_at   timestamptz NOT NULL,
    ended_at     timestamptz CHECK (ended_at >= started_at),
    FOREIGN KEY (workspace_id, component_id) REFERENCES status_components (workspace_id, id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX status_component_states_open_idx ON status_component_states (workspace_id, component_id) WHERE ended_at IS NULL;
CREATE INDEX status_component_states_span_idx ON status_component_states (workspace_id, component_id, started_at);

CREATE TABLE status_incidents (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    page_id         uuid NOT NULL,
    kind            text NOT NULL CHECK (kind IN ('incident', 'maintenance')),
    title           text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    impact          text NOT NULL CHECK (impact IN ('none', 'minor', 'major', 'critical')),
    state           text NOT NULL,
    scheduled_start timestamptz,
    scheduled_end   timestamptz,
    published_at    timestamptz,
    started_at      timestamptz,
    resolved_at     timestamptz,
    created_by      uuid,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    CHECK ((kind = 'incident' AND state IN ('investigating', 'identified', 'monitoring', 'resolved'))
        OR (kind = 'maintenance' AND state IN ('scheduled', 'in_progress', 'completed')
            AND scheduled_start IS NOT NULL AND scheduled_end > scheduled_start)),
    FOREIGN KEY (workspace_id, page_id) REFERENCES status_pages (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX status_incidents_page_idx ON status_incidents (workspace_id, page_id, created_at DESC, id DESC);
CREATE INDEX status_incidents_open_idx ON status_incidents (workspace_id, page_id) WHERE resolved_at IS NULL;

CREATE TABLE status_incident_components (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    incident_id  uuid NOT NULL,
    component_id uuid NOT NULL,
    state        text NOT NULL CHECK (state IN ('operational', 'degraded', 'partial_outage', 'major_outage', 'maintenance')),
    PRIMARY KEY (workspace_id, incident_id, component_id),
    FOREIGN KEY (workspace_id, incident_id) REFERENCES status_incidents (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, component_id) REFERENCES status_components (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX status_incident_components_component_idx ON status_incident_components (workspace_id, component_id);

CREATE TABLE status_incident_updates (
    id               uuid PRIMARY KEY,
    workspace_id     uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    incident_id      uuid NOT NULL,
    state            text NOT NULL,
    body             text NOT NULL CHECK (length(body) <= 5000),
    author_member_id uuid,
    author_key_id    uuid,
    created_at       timestamptz NOT NULL,
    edited_at        timestamptz,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, incident_id) REFERENCES status_incidents (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX status_incident_updates_incident_idx ON status_incident_updates (workspace_id, incident_id, created_at DESC);

-- +goose Down
DROP TABLE status_incident_updates;
DROP TABLE status_incident_components;
DROP TABLE status_incidents;
DROP TABLE status_component_states;
DROP TABLE status_components;
DROP TABLE status_component_groups;
DROP TABLE status_pages;
