-- Government Template Platform V3.0
-- 18_users_google_sub-ийг буцаана.

DROP INDEX IF EXISTS idx_users_google_sub_active;
ALTER TABLE users DROP COLUMN IF EXISTS google_sub;
