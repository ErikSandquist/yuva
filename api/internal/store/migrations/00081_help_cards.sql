-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE messages ADD COLUMN IF NOT EXISTS help_cards jsonb;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS help_article_id uuid;

-- +goose StatementBegin
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'messages_help_cards_check') THEN
        ALTER TABLE messages ADD CONSTRAINT messages_help_cards_check CHECK (
            help_cards IS NULL OR (jsonb_typeof(help_cards) = 'array' AND jsonb_array_length(help_cards) BETWEEN 1 AND 3)
        ) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'conversations_help_article_fk') THEN
        ALTER TABLE conversations ADD CONSTRAINT conversations_help_article_fk FOREIGN KEY (workspace_id, help_article_id)
            REFERENCES help_articles (workspace_id, id) ON DELETE SET NULL (help_article_id) NOT VALID;
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE messages VALIDATE CONSTRAINT messages_help_cards_check;
ALTER TABLE conversations VALIDATE CONSTRAINT conversations_help_article_fk;

DROP INDEX CONCURRENTLY IF EXISTS conversations_help_article_idx;
CREATE INDEX CONCURRENTLY IF NOT EXISTS conversations_help_article_idx ON conversations (workspace_id, help_article_id) WHERE help_article_id IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS conversations_help_article_idx;
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_help_article_fk;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_help_cards_check;
ALTER TABLE conversations DROP COLUMN IF EXISTS help_article_id;
ALTER TABLE messages DROP COLUMN IF EXISTS help_cards;
