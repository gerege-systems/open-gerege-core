-- Government Template Platform V3.0
DROP INDEX IF EXISTS idx_users_sso_sub;
ALTER TABLE users DROP COLUMN IF EXISTS sso_sub;
