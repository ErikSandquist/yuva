-- +goose Up
ALTER TABLE login_codes ADD COLUMN locale text CHECK (locale IN ('en', 'tr'));

-- +goose Down
ALTER TABLE login_codes DROP COLUMN locale;
