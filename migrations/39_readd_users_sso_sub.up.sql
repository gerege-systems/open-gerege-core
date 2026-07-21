-- Government Template Platform V3.0
-- Government SSO (sso.dgov.mn, OIDC) RP нэвтрэлтэд шаардлагатай sso_sub identity
-- баганыг сэргээнэ. (Өмнөх 38 drop-migration-ийг буцаах — RP login дахин идэвхжив.)
-- sso.dgov.mn нь "openid profile email" scope дор pairwise sub буцаадаг тул
-- хэрэглэгчийг sso_sub-ээр түлхүүрлэн upsert хийнэ.
ALTER TABLE users ADD COLUMN IF NOT EXISTS sso_sub TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_sso_sub
    ON users (sso_sub) WHERE sso_sub IS NOT NULL;
