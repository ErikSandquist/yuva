-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE IF NOT EXISTS roadmap_boards (
    id                    uuid PRIMARY KEY,
    workspace_id          uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    inbox_id              uuid NOT NULL,
    channel_id            uuid NOT NULL,
    name                  text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    description           text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
    slug                  text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(slug) BETWEEN 3 AND 48),
    public                boolean NOT NULL DEFAULT false,
    suggestions           text NOT NULL DEFAULT 'moderated' CHECK (suggestions IN ('off', 'moderated', 'open')),
    analytics_property_id uuid,
    -- changed_at moves with every change the public side shows except votes, so cached answers can be checked.
    changed_at            timestamptz NOT NULL,
    created_at            timestamptz NOT NULL,
    updated_at            timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, inbox_id),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, channel_id) REFERENCES channels (workspace_id, id),
    FOREIGN KEY (workspace_id, analytics_property_id) REFERENCES analytics_properties (workspace_id, id) ON DELETE SET NULL (analytics_property_id)
);
CREATE INDEX IF NOT EXISTS roadmap_boards_list_idx ON roadmap_boards (workspace_id, created_at, id);

CREATE TABLE IF NOT EXISTS roadmap_board_slugs (
    slug         text PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    board_id     uuid NOT NULL,
    created_at   timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, board_id) REFERENCES roadmap_boards (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS roadmap_board_slugs_board_idx ON roadmap_board_slugs (workspace_id, board_id, created_at);

ALTER TABLE discussions
    ADD COLUMN IF NOT EXISTS kind             text NOT NULL DEFAULT 'page',
    ADD COLUMN IF NOT EXISTS board_id         uuid,
    ADD COLUMN IF NOT EXISTS stage            text,
    ADD COLUMN IF NOT EXISTS stage_changed_at timestamptz,
    ADD COLUMN IF NOT EXISTS vote_count       integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS search           tsvector;
ALTER TABLE discussions ALTER COLUMN page DROP NOT NULL;

-- +goose StatementBegin
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'discussions_kind_check') THEN
        ALTER TABLE discussions ADD CONSTRAINT discussions_kind_check CHECK (kind IN ('page', 'idea')) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'discussions_kind_page_check') THEN
        ALTER TABLE discussions ADD CONSTRAINT discussions_kind_page_check CHECK ((kind = 'page') = (page IS NOT NULL)) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'discussions_kind_board_check') THEN
        ALTER TABLE discussions ADD CONSTRAINT discussions_kind_board_check CHECK ((kind = 'idea') = (board_id IS NOT NULL)) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'discussions_kind_stage_check') THEN
        ALTER TABLE discussions ADD CONSTRAINT discussions_kind_stage_check CHECK ((kind = 'idea') = (stage IS NOT NULL)) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'discussions_stage_check') THEN
        ALTER TABLE discussions ADD CONSTRAINT discussions_stage_check CHECK (stage IS NULL OR stage IN ('ideas', 'planned', 'in_progress', 'shipped', 'closed')) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'discussions_vote_count_check') THEN
        ALTER TABLE discussions ADD CONSTRAINT discussions_vote_count_check CHECK (vote_count >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'discussions_board_fk') THEN
        ALTER TABLE discussions ADD CONSTRAINT discussions_board_fk FOREIGN KEY (workspace_id, board_id)
            REFERENCES roadmap_boards (workspace_id, id) ON DELETE CASCADE NOT VALID;
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE discussions VALIDATE CONSTRAINT discussions_kind_check;
ALTER TABLE discussions VALIDATE CONSTRAINT discussions_kind_page_check;
ALTER TABLE discussions VALIDATE CONSTRAINT discussions_kind_board_check;
ALTER TABLE discussions VALIDATE CONSTRAINT discussions_kind_stage_check;
ALTER TABLE discussions VALIDATE CONSTRAINT discussions_stage_check;
ALTER TABLE discussions VALIDATE CONSTRAINT discussions_vote_count_check;
ALTER TABLE discussions VALIDATE CONSTRAINT discussions_board_fk;

DROP INDEX CONCURRENTLY IF EXISTS discussions_board_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS discussions_board_idx ON discussions (workspace_id, board_id, stage, vote_count DESC, id) WHERE board_id IS NOT NULL;
DROP INDEX CONCURRENTLY IF EXISTS discussions_idea_search_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS discussions_idea_search_idx ON discussions USING gin (search) WHERE kind = 'idea';
DROP INDEX CONCURRENTLY IF EXISTS discussions_opening_post_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS discussions_opening_post_idx ON discussions (workspace_id, opening_post_id);

-- A trigger and not a generated column: a generated column rewrites the table, and only ideas are searched.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION discussion_idea_search() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.kind = 'idea' THEN
        NEW.search := to_tsvector('simple', translate(NEW.title, 'İı', 'ii'));
    ELSE
        NEW.search := NULL;
    END IF;
    RETURN NEW;
END
$$;
-- +goose StatementEnd
DROP TRIGGER IF EXISTS discussions_idea_search ON discussions;
CREATE TRIGGER discussions_idea_search
    BEFORE INSERT OR UPDATE OF title, kind ON discussions
    FOR EACH ROW EXECUTE FUNCTION discussion_idea_search();

-- Kept in the vote's own transaction; every way a vote appears or goes (merges, account deletion, cascades) passes here.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION discussion_vote_count() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE discussions SET vote_count = vote_count + 1
        WHERE workspace_id = NEW.workspace_id AND opening_post_id = NEW.post_id;
        RETURN NEW;
    END IF;
    UPDATE discussions SET vote_count = greatest(vote_count - 1, 0)
    WHERE workspace_id = OLD.workspace_id AND opening_post_id = OLD.post_id;
    RETURN OLD;
END
$$;
-- +goose StatementEnd
DROP TRIGGER IF EXISTS discussion_votes_count ON discussion_votes;
CREATE TRIGGER discussion_votes_count
    AFTER INSERT OR DELETE ON discussion_votes
    FOR EACH ROW EXECUTE FUNCTION discussion_vote_count();

-- The share lock keeps votes from committing between the count and the update; the trigger above already counts later ones.
-- +goose StatementBegin
DO $$ BEGIN
    LOCK TABLE discussion_votes IN SHARE MODE;
    UPDATE discussions d SET vote_count = (
        SELECT count(*) FROM discussion_votes v WHERE v.workspace_id = d.workspace_id AND v.post_id = d.opening_post_id
    )
    WHERE d.vote_count <> (
        SELECT count(*) FROM discussion_votes v WHERE v.workspace_id = d.workspace_id AND v.post_id = d.opening_post_id
    );
END $$;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS roadmap_idea_conversations (
    workspace_id         uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    conversation_id      uuid NOT NULL,
    idea_id              uuid NOT NULL,
    linked_by_member_id  uuid,
    linked_by_api_key_id uuid,
    linked_at            timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, conversation_id),
    FOREIGN KEY (workspace_id, idea_id) REFERENCES discussions (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, conversation_id) REFERENCES conversations (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS roadmap_idea_conversations_idea_idx ON roadmap_idea_conversations (workspace_id, idea_id, linked_at DESC, conversation_id DESC);

CREATE TABLE IF NOT EXISTS roadmap_stage_changes (
    id               uuid NOT NULL,
    workspace_id     uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    idea_id          uuid NOT NULL,
    from_stage       text NOT NULL CHECK (from_stage IN ('ideas', 'planned', 'in_progress', 'shipped', 'closed')),
    to_stage         text NOT NULL CHECK (to_stage IN ('ideas', 'planned', 'in_progress', 'shipped', 'closed')),
    member_id        uuid,
    api_key_id       uuid,
    grant_id         uuid,
    via              text NOT NULL DEFAULT '',
    comment_post_id  uuid,
    linked           integer NOT NULL DEFAULT 0,
    progress         text CHECK (progress IN ('planned', 'in_progress', 'done', 'wont_do')),
    progress_set     integer NOT NULL DEFAULT 0,
    progress_skipped integer NOT NULL DEFAULT 0,
    finished_at      timestamptz,
    created_at       timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, id),
    FOREIGN KEY (workspace_id, idea_id) REFERENCES discussions (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS roadmap_stage_changes_idea_idx ON roadmap_stage_changes (workspace_id, idea_id, created_at DESC, id DESC);

ALTER TABLE IF EXISTS status_incident_replies RENAME TO linked_replies;
ALTER TABLE IF EXISTS status_incident_reply_targets RENAME TO linked_reply_targets;

ALTER TABLE linked_replies ALTER COLUMN incident_id DROP NOT NULL;
ALTER TABLE linked_replies ADD COLUMN IF NOT EXISTS idea_id uuid;

-- +goose StatementBegin
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'linked_replies_source_check') THEN
        ALTER TABLE linked_replies ADD CONSTRAINT linked_replies_source_check CHECK (num_nonnulls(incident_id, idea_id) = 1) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'linked_replies_idea_fk') THEN
        ALTER TABLE linked_replies ADD CONSTRAINT linked_replies_idea_fk FOREIGN KEY (workspace_id, idea_id)
            REFERENCES discussions (workspace_id, id) ON DELETE CASCADE NOT VALID;
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE linked_replies VALIDATE CONSTRAINT linked_replies_source_check;
ALTER TABLE linked_replies VALIDATE CONSTRAINT linked_replies_idea_fk;

DROP INDEX CONCURRENTLY IF EXISTS linked_replies_idea_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS linked_replies_idea_idx ON linked_replies (workspace_id, idea_id, created_at DESC) WHERE idea_id IS NOT NULL;

ALTER TABLE reader_notifications DROP CONSTRAINT IF EXISTS reader_notifications_kind_check;
ALTER TABLE reader_notifications ADD CONSTRAINT reader_notifications_kind_check CHECK (kind IN ('reply', 'answer', 'stage')) NOT VALID;
ALTER TABLE reader_notifications VALIDATE CONSTRAINT reader_notifications_kind_check;

-- +goose Down
ALTER TABLE reader_notifications DROP CONSTRAINT IF EXISTS reader_notifications_kind_check;
DELETE FROM reader_notifications WHERE kind = 'stage';
ALTER TABLE reader_notifications ADD CONSTRAINT reader_notifications_kind_check CHECK (kind IN ('reply', 'answer')) NOT VALID;
ALTER TABLE reader_notifications VALIDATE CONSTRAINT reader_notifications_kind_check;

DROP INDEX CONCURRENTLY IF EXISTS linked_replies_idea_idx;
DELETE FROM linked_replies WHERE idea_id IS NOT NULL;
ALTER TABLE linked_replies DROP CONSTRAINT IF EXISTS linked_replies_source_check;
ALTER TABLE linked_replies DROP CONSTRAINT IF EXISTS linked_replies_idea_fk;
ALTER TABLE linked_replies DROP COLUMN IF EXISTS idea_id;
ALTER TABLE linked_replies ALTER COLUMN incident_id SET NOT NULL;
ALTER TABLE linked_reply_targets RENAME TO status_incident_reply_targets;
ALTER TABLE linked_replies RENAME TO status_incident_replies;

DROP TABLE IF EXISTS roadmap_stage_changes;
DROP TABLE IF EXISTS roadmap_idea_conversations;

DROP TRIGGER IF EXISTS discussion_votes_count ON discussion_votes;
DROP FUNCTION IF EXISTS discussion_vote_count();
DROP TRIGGER IF EXISTS discussions_idea_search ON discussions;
DROP FUNCTION IF EXISTS discussion_idea_search();

DROP INDEX CONCURRENTLY IF EXISTS discussions_opening_post_idx;
DROP INDEX CONCURRENTLY IF EXISTS discussions_idea_search_idx;
DROP INDEX CONCURRENTLY IF EXISTS discussions_board_idx;
DELETE FROM discussions WHERE kind = 'idea';
ALTER TABLE discussions DROP CONSTRAINT IF EXISTS discussions_board_fk;
ALTER TABLE discussions DROP CONSTRAINT IF EXISTS discussions_vote_count_check;
ALTER TABLE discussions DROP CONSTRAINT IF EXISTS discussions_stage_check;
ALTER TABLE discussions DROP CONSTRAINT IF EXISTS discussions_kind_stage_check;
ALTER TABLE discussions DROP CONSTRAINT IF EXISTS discussions_kind_board_check;
ALTER TABLE discussions DROP CONSTRAINT IF EXISTS discussions_kind_page_check;
ALTER TABLE discussions DROP CONSTRAINT IF EXISTS discussions_kind_check;
ALTER TABLE discussions
    DROP COLUMN IF EXISTS search,
    DROP COLUMN IF EXISTS vote_count,
    DROP COLUMN IF EXISTS stage_changed_at,
    DROP COLUMN IF EXISTS stage,
    DROP COLUMN IF EXISTS board_id,
    DROP COLUMN IF EXISTS kind;
ALTER TABLE discussions ALTER COLUMN page SET NOT NULL;

DROP TABLE IF EXISTS roadmap_board_slugs;
DROP TABLE IF EXISTS roadmap_boards;
