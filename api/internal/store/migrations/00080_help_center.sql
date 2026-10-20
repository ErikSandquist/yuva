-- +goose Up
CREATE TABLE help_centers (
    id                     uuid PRIMARY KEY,
    workspace_id           uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    inbox_id               uuid NOT NULL,
    name                   text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    description            text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
    slug                   text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(slug) BETWEEN 3 AND 48),
    default_locale         text NOT NULL,
    locales                text[] NOT NULL CHECK (cardinality(locales) BETWEEN 1 AND 20 AND default_locale = ANY (locales)),
    logo_url               text CHECK (length(logo_url) <= 2000),
    accent_color           text CHECK (accent_color ~ '^#[0-9a-fA-F]{6}$'),
    site_url               text CHECK (length(site_url) <= 2000),
    chat_channel_id        uuid,
    analytics_property_id  uuid,
    agents_may_publish     boolean NOT NULL DEFAULT false,
    published              boolean NOT NULL DEFAULT false,
    next_collection_number integer NOT NULL DEFAULT 1,
    next_article_number    integer NOT NULL DEFAULT 1,
    -- changed_at moves with every change the public side shows, so cached answers can be checked.
    changed_at             timestamptz NOT NULL,
    created_at             timestamptz NOT NULL,
    updated_at             timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, inbox_id),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, chat_channel_id) REFERENCES channels (workspace_id, id) ON DELETE SET NULL (chat_channel_id),
    FOREIGN KEY (workspace_id, analytics_property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE SET NULL (analytics_property_id)
);
CREATE INDEX help_centers_list_idx ON help_centers (workspace_id, created_at, id);

CREATE TABLE help_center_slugs (
    slug         text PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    center_id    uuid NOT NULL,
    created_at   timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, center_id) REFERENCES help_centers (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX help_center_slugs_center_idx ON help_center_slugs (workspace_id, center_id, created_at);

CREATE TABLE help_collections (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    center_id    uuid NOT NULL,
    parent_id    uuid,
    number       integer NOT NULL,
    icon         text CHECK (length(icon) <= 40),
    position     integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, center_id, id),
    UNIQUE (workspace_id, center_id, number),
    CHECK (parent_id <> id),
    FOREIGN KEY (workspace_id, center_id) REFERENCES help_centers (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, center_id, parent_id) REFERENCES help_collections (workspace_id, center_id, id) ON DELETE SET NULL (parent_id)
);
CREATE INDEX help_collections_parent_idx ON help_collections (workspace_id, parent_id);

CREATE TABLE help_collection_translations (
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    collection_id uuid NOT NULL,
    locale        text NOT NULL,
    name          text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    description   text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    slug          text NOT NULL,
    updated_at    timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, collection_id, locale),
    FOREIGN KEY (workspace_id, collection_id) REFERENCES help_collections (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE help_articles (
    id                    uuid PRIMARY KEY,
    workspace_id          uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    center_id             uuid NOT NULL,
    collection_id         uuid,
    number                integer NOT NULL,
    position              integer NOT NULL DEFAULT 0,
    paths                 text[] NOT NULL DEFAULT '{}' CHECK (cardinality(paths) <= 20),
    created_by_member_id  uuid,
    created_by_api_key_id uuid,
    created_at            timestamptz NOT NULL,
    updated_at            timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, center_id, number),
    FOREIGN KEY (workspace_id, center_id) REFERENCES help_centers (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, center_id, collection_id) REFERENCES help_collections (workspace_id, center_id, id) ON DELETE SET NULL (collection_id)
);
CREATE INDEX help_articles_order_idx ON help_articles (workspace_id, center_id, collection_id, position);

CREATE TABLE help_article_translations (
    workspace_id            uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    article_id              uuid NOT NULL,
    locale                  text NOT NULL,
    title                   text NOT NULL CHECK (length(title) BETWEEN 3 AND 200),
    body                    text NOT NULL DEFAULT '' CHECK (length(body) <= 100000),
    keywords                text NOT NULL DEFAULT '' CHECK (length(keywords) <= 500),
    slug                    text NOT NULL CHECK (length(slug) BETWEEN 1 AND 64),
    version                 integer NOT NULL DEFAULT 1,
    image_ids               uuid[] NOT NULL DEFAULT '{}',
    published_title         text,
    published_body          text,
    published_keywords      text,
    published_slug          text,
    published_html          text,
    published_excerpt       text,
    published_at            timestamptz,
    published_by_member_id  uuid,
    published_by_api_key_id uuid,
    has_unpublished_changes boolean NOT NULL GENERATED ALWAYS AS (
        published_at IS NOT NULL AND (title, body, keywords, slug) IS DISTINCT FROM (published_title, published_body, published_keywords, published_slug)
    ) STORED,
    -- The text search configuration of the locale's language, simple for languages without one.
    search_config           text NOT NULL CHECK (search_config ~ '^[a-z_]{1,63}$'),
    search                  tsvector NOT NULL DEFAULT ''::tsvector,
    updated_by_member_id    uuid,
    updated_by_api_key_id   uuid,
    created_at              timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, article_id, locale),
    CHECK ((published_at IS NULL) = (published_title IS NULL)),
    FOREIGN KEY (workspace_id, article_id) REFERENCES help_articles (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX help_article_translations_search_idx ON help_article_translations USING gin (search) WHERE published_at IS NOT NULL;
CREATE INDEX help_article_translations_images_idx ON help_article_translations USING gin (image_ids);

-- A trigger and not a generated column: casting search_config to regconfig is not immutable.
-- The vector holds the stemmed words of the published copy and the words as written, so a word
-- typed halfway still matches by prefix; İ and ı fold to i as in the other searches.
-- +goose StatementBegin
CREATE FUNCTION help_translation_search() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    cfg regconfig := NEW.search_config::regconfig;
BEGIN
    NEW.search :=
        setweight(to_tsvector(cfg, translate(coalesce(NEW.published_title, ''), 'İı', 'ii')), 'A') ||
        setweight(to_tsvector('simple', translate(coalesce(NEW.published_title, ''), 'İı', 'ii')), 'A') ||
        setweight(to_tsvector(cfg, translate(coalesce(NEW.published_keywords, ''), 'İı', 'ii')), 'B') ||
        setweight(to_tsvector('simple', translate(coalesce(NEW.published_keywords, ''), 'İı', 'ii')), 'B') ||
        setweight(to_tsvector(cfg, translate(coalesce(NEW.published_body, ''), 'İı', 'ii')), 'C') ||
        setweight(to_tsvector('simple', translate(coalesce(NEW.published_body, ''), 'İı', 'ii')), 'C');
    RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER help_article_translations_search
    BEFORE INSERT OR UPDATE OF published_title, published_keywords, published_body, search_config ON help_article_translations
    FOR EACH ROW EXECUTE FUNCTION help_translation_search();

CREATE TABLE help_article_revisions (
    id                uuid PRIMARY KEY,
    workspace_id      uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    article_id        uuid NOT NULL,
    locale            text NOT NULL,
    kind              text NOT NULL CHECK (kind IN ('save', 'publish', 'restore')),
    title             text NOT NULL,
    body              text NOT NULL,
    keywords          text NOT NULL,
    image_ids         uuid[] NOT NULL DEFAULT '{}',
    author_member_id  uuid,
    author_api_key_id uuid,
    via               text,
    created_at        timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, article_id, locale) REFERENCES help_article_translations (workspace_id, article_id, locale) ON DELETE CASCADE
);
CREATE INDEX help_article_revisions_idx ON help_article_revisions (workspace_id, article_id, locale, created_at DESC, id DESC);
CREATE INDEX help_article_revisions_images_idx ON help_article_revisions USING gin (image_ids);

CREATE TABLE help_article_daily (
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    center_id     uuid NOT NULL,
    article_id    uuid NOT NULL,
    locale        text NOT NULL,
    day           date NOT NULL,
    views         integer NOT NULL DEFAULT 0,
    helpful_up    integer NOT NULL DEFAULT 0,
    helpful_down  integer NOT NULL DEFAULT 0,
    conversations integer NOT NULL DEFAULT 0,
    sent          integer NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_id, article_id, locale, day),
    FOREIGN KEY (workspace_id, center_id) REFERENCES help_centers (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, article_id) REFERENCES help_articles (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX help_article_daily_center_idx ON help_article_daily (workspace_id, center_id, day);

CREATE TABLE help_search_daily (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    center_id    uuid NOT NULL,
    locale       text NOT NULL,
    day          date NOT NULL,
    query        text NOT NULL CHECK (length(query) BETWEEN 1 AND 100),
    searches     integer NOT NULL DEFAULT 0,
    no_results   integer NOT NULL DEFAULT 0,
    clicks       integer NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_id, center_id, locale, day, query),
    FOREIGN KEY (workspace_id, center_id) REFERENCES help_centers (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX help_search_daily_day_idx ON help_search_daily (workspace_id, day);

ALTER TABLE usage_counters ADD COLUMN help_searches bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE usage_counters DROP COLUMN help_searches;
DROP TABLE help_search_daily;
DROP TABLE help_article_daily;
DROP TABLE help_article_revisions;
DROP TABLE help_article_translations;
DROP FUNCTION help_translation_search();
DROP TABLE help_articles;
DROP TABLE help_collection_translations;
DROP TABLE help_collections;
DROP TABLE help_center_slugs;
DROP TABLE help_centers;
