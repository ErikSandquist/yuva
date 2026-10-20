-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS changelogs (
    id                    uuid PRIMARY KEY,
    workspace_id          uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    inbox_id              uuid NOT NULL,
    name                  text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    description           text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
    slug                  text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(slug) BETWEEN 3 AND 48),
    default_locale        text NOT NULL,
    locales               text[] NOT NULL CHECK (cardinality(locales) BETWEEN 1 AND 20 AND default_locale = ANY (locales)),
    analytics_property_id uuid,
    agents_may_publish    boolean NOT NULL DEFAULT false,
    published             boolean NOT NULL DEFAULT false,
    next_entry_number     integer NOT NULL DEFAULT 1,
    -- changed_at moves with every change the public side shows, so cached answers can be checked.
    changed_at            timestamptz NOT NULL,
    created_at            timestamptz NOT NULL,
    updated_at            timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, inbox_id),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, analytics_property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE SET NULL (analytics_property_id)
);
CREATE INDEX IF NOT EXISTS changelogs_list_idx ON changelogs (workspace_id, created_at, id);

CREATE TABLE IF NOT EXISTS changelog_slugs (
    slug         text PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    changelog_id uuid NOT NULL,
    created_at   timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, changelog_id) REFERENCES changelogs (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS changelog_slugs_changelog_idx ON changelog_slugs (workspace_id, changelog_id, created_at);

CREATE TABLE IF NOT EXISTS changelog_entries (
    id                      uuid PRIMARY KEY,
    workspace_id            uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    changelog_id            uuid NOT NULL,
    number                  integer NOT NULL,
    labels                  text[] NOT NULL CHECK (cardinality(labels) BETWEEN 1 AND 3 AND labels <@ ARRAY['new', 'improved', 'fixed']),
    image_id                uuid,
    state                   text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'scheduled', 'published')),
    publish_at              timestamptz,
    published_at            timestamptz,
    scheduled_by_member_id  uuid,
    scheduled_by_api_key_id uuid,
    schedule_failed         text CHECK (schedule_failed IN ('lost_right', 'key_revoked', 'key_scope_lost', 'no_default_translation')),
    schedule_failed_at      timestamptz,
    created_by_member_id    uuid,
    created_by_api_key_id   uuid,
    created_at              timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, changelog_id, number),
    CHECK ((state = 'scheduled') = (publish_at IS NOT NULL)),
    CHECK (state <> 'published' OR published_at IS NOT NULL),
    CHECK ((schedule_failed IS NULL) = (schedule_failed_at IS NULL)),
    FOREIGN KEY (workspace_id, changelog_id) REFERENCES changelogs (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, image_id) REFERENCES email_images (workspace_id, id) ON DELETE SET NULL (image_id)
);
CREATE INDEX IF NOT EXISTS changelog_entries_published_idx ON changelog_entries (workspace_id, changelog_id, published_at DESC, id) WHERE state = 'published';
CREATE INDEX IF NOT EXISTS changelog_entries_due_idx ON changelog_entries (publish_at) WHERE state = 'scheduled';
CREATE INDEX IF NOT EXISTS changelog_entries_image_idx ON changelog_entries (workspace_id, image_id) WHERE image_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS changelog_entry_translations (
    workspace_id            uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    entry_id                uuid NOT NULL,
    locale                  text NOT NULL,
    title                   text NOT NULL CHECK (length(title) BETWEEN 3 AND 200),
    body                    text NOT NULL DEFAULT '' CHECK (length(body) <= 50000),
    slug                    text NOT NULL CHECK (length(slug) BETWEEN 1 AND 64),
    version                 integer NOT NULL DEFAULT 1,
    image_ids               uuid[] NOT NULL DEFAULT '{}',
    published_title         text,
    published_body          text,
    published_slug          text,
    published_html          text,
    published_excerpt       text,
    published_at            timestamptz,
    published_by_member_id  uuid,
    published_by_api_key_id uuid,
    has_unpublished_changes boolean NOT NULL GENERATED ALWAYS AS (
        published_at IS NOT NULL AND (title, body, slug) IS DISTINCT FROM (published_title, published_body, published_slug)
    ) STORED,
    updated_by_member_id    uuid,
    updated_by_api_key_id   uuid,
    created_at              timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, entry_id, locale),
    CHECK ((published_at IS NULL) = (published_title IS NULL)),
    FOREIGN KEY (workspace_id, entry_id) REFERENCES changelog_entries (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS changelog_entry_translations_images_idx ON changelog_entry_translations USING gin (image_ids);

ALTER TABLE chat_channels ADD COLUMN IF NOT EXISTS announce_updates boolean NOT NULL DEFAULT false;
ALTER TABLE broadcasts ADD COLUMN IF NOT EXISTS changelog_entry_id uuid;

-- +goose StatementBegin
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'broadcasts_changelog_entry_fk') THEN
        ALTER TABLE broadcasts ADD CONSTRAINT broadcasts_changelog_entry_fk FOREIGN KEY (workspace_id, changelog_entry_id)
            REFERENCES changelog_entries (workspace_id, id) ON DELETE SET NULL (changelog_entry_id) NOT VALID;
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE broadcasts VALIDATE CONSTRAINT broadcasts_changelog_entry_fk;

DROP INDEX CONCURRENTLY IF EXISTS broadcasts_changelog_entry_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS broadcasts_changelog_entry_idx ON broadcasts (workspace_id, changelog_entry_id) WHERE changelog_entry_id IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS broadcasts_changelog_entry_idx;
ALTER TABLE broadcasts DROP CONSTRAINT IF EXISTS broadcasts_changelog_entry_fk;
ALTER TABLE broadcasts DROP COLUMN IF EXISTS changelog_entry_id;
ALTER TABLE chat_channels DROP COLUMN IF EXISTS announce_updates;
DROP TABLE IF EXISTS changelog_entry_translations;
DROP TABLE IF EXISTS changelog_entries;
DROP TABLE IF EXISTS changelog_slugs;
DROP TABLE IF EXISTS changelogs;
