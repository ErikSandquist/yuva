-- +goose Up
ALTER TABLE chat_channels
    ADD COLUMN discussions text NOT NULL DEFAULT 'moderated' CHECK (discussions IN ('off', 'moderated', 'open'));
UPDATE chat_channels cc SET discussions = 'off'
FROM channels c
WHERE c.workspace_id = cc.workspace_id AND c.id = cc.channel_id AND c.kind <> 'chat';

ALTER TABLE people ADD COLUMN reader_email_notifications boolean NOT NULL DEFAULT true;

ALTER TABLE login_codes ADD COLUMN reader boolean NOT NULL DEFAULT false;

ALTER TABLE contacts
    ADD COLUMN person_id              uuid REFERENCES people (id) ON DELETE SET NULL,
    ADD COLUMN discussions_blocked_at timestamptz;
CREATE UNIQUE INDEX contacts_person_idx ON contacts (workspace_id, person_id) WHERE person_id IS NOT NULL;
CREATE INDEX contacts_person_id_idx ON contacts (person_id) WHERE person_id IS NOT NULL;

CREATE TABLE discussions (
    id               uuid PRIMARY KEY,
    workspace_id     uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    inbox_id         uuid NOT NULL,
    channel_id       uuid NOT NULL,
    page             text NOT NULL CHECK (length(page) BETWEEN 1 AND 2000),
    page_title       text NOT NULL DEFAULT '' CHECK (length(page_title) <= 300),
    title            text NOT NULL CHECK (length(title) BETWEEN 3 AND 200),
    opening_post_id  uuid NOT NULL,
    state            text NOT NULL CHECK (state IN ('published', 'pending', 'hidden')),
    locked_at        timestamptz,
    answer_post_id   uuid,
    answered_at      timestamptz,
    reply_count      integer NOT NULL DEFAULT 0,
    pending_posts    integer NOT NULL DEFAULT 0,
    hidden_posts     integer NOT NULL DEFAULT 0,
    conversation_id  uuid,
    created_at       timestamptz NOT NULL,
    last_activity_at timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, channel_id) REFERENCES channels (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, conversation_id) REFERENCES conversations (workspace_id, id) ON DELETE SET NULL (conversation_id)
);
CREATE INDEX discussions_page_idx ON discussions (workspace_id, inbox_id, page, created_at DESC);
CREATE INDEX discussions_activity_idx ON discussions (workspace_id, last_activity_at DESC, id DESC);
CREATE UNIQUE INDEX discussions_conversation_idx ON discussions (workspace_id, conversation_id) WHERE conversation_id IS NOT NULL;

CREATE TABLE discussion_posts (
    id                uuid PRIMARY KEY,
    workspace_id      uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    discussion_id     uuid NOT NULL,
    parent_post_id    uuid,
    opening           boolean NOT NULL DEFAULT false,
    author_type       text NOT NULL CHECK (author_type IN ('member', 'contact', 'anonymous')),
    author_member_id  uuid,
    author_contact_id uuid,
    body              text NOT NULL CHECK (length(body) <= 20000),
    state             text NOT NULL CHECK (state IN ('published', 'pending', 'hidden')),
    created_at        timestamptz NOT NULL,
    edited_at         timestamptz,
    deleted_at        timestamptz,
    UNIQUE (workspace_id, id),
    CHECK (NOT (opening AND parent_post_id IS NOT NULL)),
    FOREIGN KEY (workspace_id, discussion_id) REFERENCES discussions (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, parent_post_id) REFERENCES discussion_posts (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, author_member_id) REFERENCES members (workspace_id, id) ON DELETE SET NULL (author_member_id),
    FOREIGN KEY (workspace_id, author_contact_id) REFERENCES contacts (workspace_id, id) ON DELETE SET NULL (author_contact_id)
);
CREATE INDEX discussion_posts_thread_idx ON discussion_posts (workspace_id, discussion_id, created_at, id);
CREATE INDEX discussion_posts_state_idx ON discussion_posts (workspace_id, state, created_at, id) WHERE state <> 'published';
CREATE INDEX discussion_posts_contact_idx ON discussion_posts (workspace_id, author_contact_id) WHERE author_contact_id IS NOT NULL;

ALTER TABLE discussions
    ADD FOREIGN KEY (workspace_id, opening_post_id) REFERENCES discussion_posts (workspace_id, id) DEFERRABLE INITIALLY DEFERRED,
    ADD FOREIGN KEY (workspace_id, answer_post_id) REFERENCES discussion_posts (workspace_id, id) ON DELETE SET NULL (answer_post_id);

CREATE TABLE discussion_votes (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    post_id      uuid NOT NULL,
    contact_id   uuid,
    member_id    uuid,
    created_at   timestamptz NOT NULL,
    CHECK ((contact_id IS NULL) <> (member_id IS NULL)),
    FOREIGN KEY (workspace_id, post_id) REFERENCES discussion_posts (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, contact_id) REFERENCES contacts (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, member_id) REFERENCES members (workspace_id, id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX discussion_votes_contact_idx ON discussion_votes (workspace_id, post_id, contact_id) WHERE contact_id IS NOT NULL;
CREATE UNIQUE INDEX discussion_votes_member_idx ON discussion_votes (workspace_id, post_id, member_id) WHERE member_id IS NOT NULL;
CREATE INDEX discussion_votes_voter_idx ON discussion_votes (workspace_id, contact_id) WHERE contact_id IS NOT NULL;

CREATE TABLE discussion_follows (
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    discussion_id uuid NOT NULL,
    contact_id    uuid NOT NULL,
    created_at    timestamptz NOT NULL,
    seen_through  timestamptz NOT NULL,
    emailed_at    timestamptz,
    PRIMARY KEY (workspace_id, discussion_id, contact_id),
    FOREIGN KEY (workspace_id, discussion_id) REFERENCES discussions (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, contact_id) REFERENCES contacts (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX discussion_follows_contact_idx ON discussion_follows (workspace_id, contact_id);

CREATE TABLE reader_tokens (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id   uuid NOT NULL,
    person_id    uuid NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    session_id   uuid NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    token_hash   bytea NOT NULL UNIQUE,
    origin       text NOT NULL,
    created_at   timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL,
    FOREIGN KEY (workspace_id, channel_id) REFERENCES channels (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX reader_tokens_person_idx ON reader_tokens (person_id);
CREATE INDEX reader_tokens_session_idx ON reader_tokens (session_id);

CREATE TABLE reader_notifications (
    id            uuid PRIMARY KEY,
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    person_id     uuid NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    discussion_id uuid NOT NULL,
    post_id       uuid,
    kind          text NOT NULL CHECK (kind IN ('reply', 'answer')),
    created_at    timestamptz NOT NULL,
    read_at       timestamptz,
    FOREIGN KEY (workspace_id, discussion_id) REFERENCES discussions (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, post_id) REFERENCES discussion_posts (workspace_id, id) ON DELETE SET NULL (post_id)
);
CREATE INDEX reader_notifications_person_idx ON reader_notifications (person_id, created_at DESC, id DESC);
CREATE INDEX reader_notifications_age_idx ON reader_notifications (created_at);

CREATE VIEW discussion_post_view AS
SELECT p.id, p.workspace_id, p.discussion_id, p.parent_post_id, p.opening, p.author_type, p.author_member_id,
       p.author_contact_id, p.body, p.state, p.created_at, p.edited_at, p.deleted_at,
       (SELECT count(*) FROM discussion_votes v WHERE v.workspace_id = p.workspace_id AND v.post_id = p.id)::integer AS votes,
       coalesce(CASE
           WHEN p.author_type = 'member' THEN mp.name
           WHEN c.person_id IS NOT NULL THEN cp.name
           ELSE c.name
       END, '')::text AS author_name,
       c.person_id AS author_person_id
FROM discussion_posts p
LEFT JOIN members m ON m.workspace_id = p.workspace_id AND m.id = p.author_member_id
LEFT JOIN people mp ON mp.id = m.person_id
LEFT JOIN contacts c ON c.workspace_id = p.workspace_id AND c.id = p.author_contact_id
LEFT JOIN people cp ON cp.id = c.person_id;

CREATE TEMP TABLE m12_answers ON COMMIT DROP AS
SELECT a.*, gen_random_uuid() AS discussion_id, gen_random_uuid() AS question_id, gen_random_uuid() AS answer_id,
       left(btrim(split_part(btrim(a.question), E'\n', 1)), 200) AS first_line
FROM page_answers a;

INSERT INTO discussions (id, workspace_id, inbox_id, channel_id, page, page_title, title, opening_post_id, state,
                         reply_count, conversation_id, created_at, last_activity_at)
SELECT discussion_id, workspace_id, inbox_id, channel_id, page, title,
       CASE WHEN length(first_line) >= 3 THEN first_line ELSE 'Question' END,
       question_id, 'published', 1, conversation_id, published_at, updated_at
FROM m12_answers;

INSERT INTO discussion_posts (id, workspace_id, discussion_id, opening, author_type, body, state, created_at)
SELECT question_id, workspace_id, discussion_id, true, 'anonymous', question, 'published', published_at
FROM m12_answers;

INSERT INTO discussion_posts (id, workspace_id, discussion_id, opening, author_type, author_member_id, body, state, created_at, edited_at)
SELECT answer_id, workspace_id, discussion_id, false, CASE WHEN member_id IS NULL THEN 'anonymous' ELSE 'member' END,
       member_id, answer, 'published', published_at, nullif(updated_at, published_at)
FROM m12_answers;

UPDATE discussions d SET answer_post_id = m.answer_id, answered_at = m.published_at
FROM m12_answers m
WHERE d.workspace_id = m.workspace_id AND d.id = m.discussion_id;

DROP TABLE page_answers;

UPDATE conversations SET kind = 'conversation' WHERE kind = 'question';
ALTER TABLE conversations
    DROP CONSTRAINT conversations_question_check,
    DROP CONSTRAINT conversations_kind_check,
    ADD CONSTRAINT conversations_kind_check CHECK (kind IN ('conversation', 'feedback'));

-- +goose Down
ALTER TABLE conversations
    DROP CONSTRAINT conversations_kind_check,
    ADD CONSTRAINT conversations_kind_check CHECK (kind IN ('conversation', 'feedback', 'question')),
    ADD CONSTRAINT conversations_question_check CHECK (kind <> 'question' OR page_url IS NOT NULL);

CREATE TABLE page_answers (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    inbox_id        uuid NOT NULL,
    channel_id      uuid NOT NULL,
    page            text NOT NULL CHECK (length(page) BETWEEN 1 AND 2000),
    title           text NOT NULL DEFAULT '' CHECK (length(title) <= 300),
    question        text NOT NULL CHECK (length(question) BETWEEN 1 AND 2000),
    answer          text NOT NULL CHECK (length(answer) BETWEEN 1 AND 20000),
    member_id       uuid,
    conversation_id uuid,
    published_at    timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, channel_id) REFERENCES channels (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, member_id) REFERENCES members (workspace_id, id) ON DELETE SET NULL (member_id),
    FOREIGN KEY (workspace_id, conversation_id) REFERENCES conversations (workspace_id, id) ON DELETE SET NULL (conversation_id)
);
CREATE INDEX page_answers_page_idx ON page_answers (workspace_id, inbox_id, page, published_at DESC, id DESC);
CREATE INDEX page_answers_list_idx ON page_answers (workspace_id, published_at DESC, id DESC);
CREATE UNIQUE INDEX page_answers_conversation_idx ON page_answers (workspace_id, conversation_id) WHERE conversation_id IS NOT NULL;

DROP VIEW discussion_post_view;
DROP TABLE reader_notifications;
DROP TABLE reader_tokens;
DROP TABLE discussion_follows;
DROP TABLE discussion_votes;
ALTER TABLE discussions DROP CONSTRAINT discussions_workspace_id_opening_post_id_fkey,
    DROP CONSTRAINT discussions_workspace_id_answer_post_id_fkey;
DROP TABLE discussion_posts;
DROP TABLE discussions;
DROP INDEX contacts_person_id_idx;
DROP INDEX contacts_person_idx;
ALTER TABLE contacts DROP COLUMN discussions_blocked_at, DROP COLUMN person_id;
ALTER TABLE login_codes DROP COLUMN reader;
ALTER TABLE people DROP COLUMN reader_email_notifications;
ALTER TABLE chat_channels DROP COLUMN discussions;
