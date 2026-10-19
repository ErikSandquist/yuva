-- +goose Up
ALTER TABLE status_pages ADD COLUMN custom_domain text
    CHECK (custom_domain = lower(custom_domain) AND length(custom_domain) BETWEEN 4 AND 253);
ALTER TABLE status_pages ADD CONSTRAINT status_pages_custom_domain_key UNIQUE (custom_domain);
ALTER TABLE status_pages ADD COLUMN domain_token text;
ALTER TABLE status_pages ADD COLUMN domain_state text CHECK (domain_state IN ('pending', 'verified', 'failed', 'off'));
ALTER TABLE status_pages ADD COLUMN domain_problem text;
ALTER TABLE status_pages ADD COLUMN domain_checked_at timestamptz;
ALTER TABLE status_pages ADD COLUMN domain_check_at timestamptz;
ALTER TABLE status_pages ADD COLUMN domain_verified_at timestamptz;
ALTER TABLE status_pages ADD COLUMN domain_missing_since timestamptz;
ALTER TABLE status_pages ADD COLUMN cf_hostname_id text;
ALTER TABLE status_pages ADD COLUMN cf_status text;
ALTER TABLE status_pages ADD COLUMN cf_ssl_status text;
ALTER TABLE status_pages ADD COLUMN cf_errors text[];
ALTER TABLE status_pages ADD COLUMN cf_checked_at timestamptz;
ALTER TABLE status_pages ADD CONSTRAINT status_pages_domain_complete
    CHECK ((custom_domain IS NULL) = (domain_state IS NULL) AND (custom_domain IS NULL) = (domain_token IS NULL));
CREATE INDEX status_pages_domain_due_idx ON status_pages (domain_check_at) WHERE domain_check_at IS NOT NULL;

-- +goose Down
DROP INDEX status_pages_domain_due_idx;
ALTER TABLE status_pages DROP CONSTRAINT status_pages_domain_complete;
ALTER TABLE status_pages DROP CONSTRAINT status_pages_custom_domain_key;
ALTER TABLE status_pages DROP COLUMN cf_checked_at;
ALTER TABLE status_pages DROP COLUMN cf_errors;
ALTER TABLE status_pages DROP COLUMN cf_ssl_status;
ALTER TABLE status_pages DROP COLUMN cf_status;
ALTER TABLE status_pages DROP COLUMN cf_hostname_id;
ALTER TABLE status_pages DROP COLUMN domain_missing_since;
ALTER TABLE status_pages DROP COLUMN domain_verified_at;
ALTER TABLE status_pages DROP COLUMN domain_check_at;
ALTER TABLE status_pages DROP COLUMN domain_checked_at;
ALTER TABLE status_pages DROP COLUMN domain_problem;
ALTER TABLE status_pages DROP COLUMN domain_state;
ALTER TABLE status_pages DROP COLUMN domain_token;
ALTER TABLE status_pages DROP COLUMN custom_domain;
