-- Government Template Platform V3.0
-- Revert the sso_tokens table (and its RLS policies).

DROP POLICY IF EXISTS sso_tokens_self ON sso_tokens;
DROP POLICY IF EXISTS sso_tokens_admin ON sso_tokens;
DROP POLICY IF EXISTS sso_tokens_service ON sso_tokens;

DROP TABLE IF EXISTS sso_tokens;
