-- +goose Up
CREATE TABLE ses_tenants (
    workspace_id      uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    inbox_id          uuid NOT NULL,
    name              text NOT NULL UNIQUE CHECK (name ~ '^[A-Za-z0-9_-]{1,64}$'),
    region            text NOT NULL CHECK (region ~ '^[a-z]{2}(-[a-z]+)+-[0-9]$'),
    arn               text UNIQUE CHECK (length(arn) BETWEEN 1 AND 512),
    state             text NOT NULL DEFAULT 'creating' CHECK (state IN ('creating', 'ready', 'error', 'missing')),
    error             text CHECK (length(error) <= 2000),
    sending_status    text NOT NULL DEFAULT 'enabled' CHECK (sending_status IN ('enabled', 'reinstated', 'disabled')),
    customer_status   text CHECK (customer_status IN ('enabled', 'reinstated', 'disabled')),
    aws_status        text CHECK (aws_status IN ('enabled', 'reinstated', 'disabled')),
    cause             text CHECK (length(cause) <= 2000),
    reputation_impact text NOT NULL DEFAULT 'none' CHECK (reputation_impact IN ('none', 'low', 'high')),
    status_at         timestamptz,
    checked_at        timestamptz,
    created_at        timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, inbox_id),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES inboxes (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE ses_tenant_resources (
    workspace_id uuid NOT NULL,
    inbox_id     uuid NOT NULL,
    arn          text NOT NULL CHECK (length(arn) BETWEEN 1 AND 512),
    kind         text NOT NULL CHECK (kind IN ('identity', 'configuration_set')),
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 320),
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, inbox_id, arn),
    FOREIGN KEY (workspace_id, inbox_id) REFERENCES ses_tenants (workspace_id, inbox_id) ON DELETE CASCADE
);

ALTER TABLE email_channels
    ADD COLUMN ses_tenant                 text CHECK (ses_tenant ~ '^[A-Za-z0-9_-]{1,64}$'),
    ADD COLUMN ses_configuration_set      text CHECK (ses_configuration_set ~ '^[A-Za-z0-9_-]{1,64}$'),
    ADD COLUMN smtp_aws_account           text CHECK (smtp_aws_account ~ '^[0-9]{12}$'),
    ADD COLUMN broadcast_smtp_aws_account text CHECK (broadcast_smtp_aws_account ~ '^[0-9]{12}$'),
    ADD COLUMN ses_state                  text NOT NULL DEFAULT 'none'
        CHECK (ses_state IN ('none', 'pending', 'isolated', 'identity_missing', 'not_isolated', 'error')),
    ADD COLUMN ses_detail                 text CHECK (length(ses_detail) <= 2000),
    ADD COLUMN ses_checked_at             timestamptz;

ALTER TABLE broadcasts
    DROP CONSTRAINT broadcasts_pause_reason_check,
    ADD CONSTRAINT broadcasts_pause_reason_check
        CHECK (pause_reason IN ('member', 'bounce_rate', 'complaint_rate', 'sender_error', 'workspace_suspended', 'operator', 'ses_tenant'));

ALTER TABLE operator_notices
    DROP CONSTRAINT operator_notices_kind_check,
    ADD CONSTRAINT operator_notices_kind_check
        CHECK (kind IN ('signup', 'workspace', 'broadcast_approval', 'broadcast_suspended', 'broadcast_paused', 'analytics_limit', 'analytics_dropped', 'ses_tenant_paused'));

-- +goose Down
DELETE FROM operator_notices WHERE kind = 'ses_tenant_paused';
ALTER TABLE operator_notices
    DROP CONSTRAINT operator_notices_kind_check,
    ADD CONSTRAINT operator_notices_kind_check
        CHECK (kind IN ('signup', 'workspace', 'broadcast_approval', 'broadcast_suspended', 'broadcast_paused', 'analytics_limit', 'analytics_dropped'));
UPDATE broadcasts SET pause_reason = 'operator' WHERE pause_reason = 'ses_tenant';
ALTER TABLE broadcasts
    DROP CONSTRAINT broadcasts_pause_reason_check,
    ADD CONSTRAINT broadcasts_pause_reason_check
        CHECK (pause_reason IN ('member', 'bounce_rate', 'complaint_rate', 'sender_error', 'workspace_suspended', 'operator'));
ALTER TABLE email_channels
    DROP COLUMN ses_checked_at, DROP COLUMN ses_detail, DROP COLUMN ses_state,
    DROP COLUMN broadcast_smtp_aws_account, DROP COLUMN smtp_aws_account,
    DROP COLUMN ses_configuration_set, DROP COLUMN ses_tenant;
DROP TABLE ses_tenant_resources;
DROP TABLE ses_tenants;
