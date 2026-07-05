-- Gerege Template Version 27.0
-- Gerege SSO (sso.gerege.mn, OIDC) нэвтрэлт — pairwise subject (sub)-ээр
-- хэрэглэгчийг таньж холбоно. sso.gerege.mn нь scope "openid profile email"
-- дор national_id/civil_id буцаадаггүй (pairwise sub) тул eID хэрэглэгчтэй
-- регистрээр нэгтгэх боломжгүй — SSO нэвтрэлт нь sso_sub-ээр түлхүүрлэгдсэн
-- тусдаа identity. Хожим eID-ээр баталгаажуулж нэгтгэх боломжийг үлдээв.
ALTER TABLE users ADD COLUMN IF NOT EXISTS sso_sub TEXT;

-- Нэг pairwise sub = нэг идэвхтэй хэрэглэгч. NULL (SSO-гүй) мөрүүдэд хамаарахгүй.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_sso_sub
    ON users (sso_sub) WHERE sso_sub IS NOT NULL;
