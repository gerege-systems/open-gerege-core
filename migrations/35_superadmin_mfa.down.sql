-- Government Template Platform V3.0
-- 35_superadmin_mfa-ийн буцаалт.
DROP TABLE IF EXISTS superadmin_invites;
DROP TABLE IF EXISTS user_recovery_codes;
ALTER TABLE users
    DROP COLUMN IF EXISTS email_verified,
    DROP COLUMN IF EXISTS mfa_enabled,
    DROP COLUMN IF EXISTS totp_secret;
