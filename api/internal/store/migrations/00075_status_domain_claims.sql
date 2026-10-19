-- +goose Up
ALTER TABLE status_pages DROP CONSTRAINT status_pages_custom_domain_key;
CREATE UNIQUE INDEX status_pages_custom_domain_verified_key ON status_pages (custom_domain) WHERE domain_state = 'verified';
CREATE INDEX status_pages_custom_domain_idx ON status_pages (custom_domain) WHERE custom_domain IS NOT NULL;
ALTER TABLE status_pages ADD COLUMN domain_claimed_at timestamptz;
UPDATE status_pages SET domain_claimed_at = updated_at WHERE custom_domain IS NOT NULL;
ALTER TABLE status_pages ADD CONSTRAINT status_pages_domain_claimed
    CHECK ((custom_domain IS NULL) = (domain_claimed_at IS NULL));

-- +goose Down
ALTER TABLE status_pages DROP CONSTRAINT status_pages_domain_claimed;
ALTER TABLE status_pages DROP COLUMN domain_claimed_at;
DROP INDEX status_pages_custom_domain_idx;
DROP INDEX status_pages_custom_domain_verified_key;
ALTER TABLE status_pages ADD CONSTRAINT status_pages_custom_domain_key UNIQUE (custom_domain);
