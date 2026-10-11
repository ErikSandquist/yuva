-- +goose Up
ALTER TABLE email_design_versions DROP CONSTRAINT email_design_versions_reason_check,
    ADD CONSTRAINT email_design_versions_reason_check
        CHECK (reason IN ('autosave', 'test', 'scheduled', 'restored', 'manual', 'conflict'));

-- +goose Down
UPDATE email_design_versions SET reason = 'manual' WHERE reason = 'conflict';
ALTER TABLE email_design_versions DROP CONSTRAINT email_design_versions_reason_check,
    ADD CONSTRAINT email_design_versions_reason_check
        CHECK (reason IN ('autosave', 'test', 'scheduled', 'restored', 'manual'));
