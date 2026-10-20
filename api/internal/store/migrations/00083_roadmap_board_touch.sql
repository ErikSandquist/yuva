-- +goose Up
-- A post that is edited, hidden, erased or detached from its author moves the board's cache key, whichever code path did it.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION roadmap_board_touch() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    ws uuid;
    did uuid;
BEGIN
    IF TG_OP = 'DELETE' THEN
        ws := OLD.workspace_id; did := OLD.discussion_id;
    ELSE
        ws := NEW.workspace_id; did := NEW.discussion_id;
    END IF;
    UPDATE roadmap_boards b SET changed_at = greatest(b.changed_at + interval '1 microsecond', now())
    FROM discussions d
    WHERE d.workspace_id = ws AND d.id = did AND d.board_id IS NOT NULL AND b.workspace_id = d.workspace_id AND b.id = d.board_id;
    RETURN NULL;
END
$$;
-- +goose StatementEnd
DROP TRIGGER IF EXISTS discussion_posts_board_touch ON discussion_posts;
CREATE TRIGGER discussion_posts_board_touch
    AFTER UPDATE OF body, state, deleted_at, author_member_id, author_contact_id OR DELETE ON discussion_posts
    FOR EACH ROW EXECUTE FUNCTION roadmap_board_touch();

-- +goose Down
DROP TRIGGER IF EXISTS discussion_posts_board_touch ON discussion_posts;
DROP FUNCTION IF EXISTS roadmap_board_touch();
