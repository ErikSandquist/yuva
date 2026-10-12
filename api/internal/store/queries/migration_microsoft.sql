-- A refreshed token replaces the one it came from, and never credentials a new sign-in stored
-- meanwhile.
-- name: ResealMigrationCredentials :execrows
UPDATE migration_credentials SET sealed = @sealed, scopes = @scopes, expires_at = sqlc.narg(expires_at)
WHERE workspace_id = @workspace_id AND migration_id = @migration_id AND sealed = @previous;
