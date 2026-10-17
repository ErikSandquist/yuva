-- +goose Up
ALTER TABLE people DROP CONSTRAINT people_locale_check;
ALTER TABLE invites DROP CONSTRAINT invites_locale_check;
ALTER TABLE login_codes DROP CONSTRAINT login_codes_locale_check;
ALTER TABLE people ADD CONSTRAINT people_locale_check CHECK (length(locale) BETWEEN 2 AND 35);
ALTER TABLE invites ADD CONSTRAINT invites_locale_check CHECK (length(locale) BETWEEN 2 AND 35);
ALTER TABLE login_codes ADD CONSTRAINT login_codes_locale_check CHECK (length(locale) BETWEEN 2 AND 35);

-- +goose Down
ALTER TABLE login_codes DROP CONSTRAINT login_codes_locale_check;
ALTER TABLE invites DROP CONSTRAINT invites_locale_check;
ALTER TABLE people DROP CONSTRAINT people_locale_check;
UPDATE login_codes SET locale = 'en' WHERE locale NOT IN ('en', 'tr');
UPDATE invites SET locale = 'en' WHERE locale NOT IN ('en', 'tr');
UPDATE people SET locale = 'en' WHERE locale NOT IN ('en', 'tr');
ALTER TABLE login_codes ADD CONSTRAINT login_codes_locale_check CHECK (locale IN ('en', 'tr'));
ALTER TABLE invites ADD CONSTRAINT invites_locale_check CHECK (locale IN ('en', 'tr'));
ALTER TABLE people ADD CONSTRAINT people_locale_check CHECK (locale IN ('en', 'tr'));
