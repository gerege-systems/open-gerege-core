-- Government Template Platform V3.0
-- Google холболт: eID хэрэглэгчийн бүртгэлд Google account-ийг холбоно. Google-ээр
-- эхний удаа нэвтрэхэд ЗААВАЛ eID-ээр баталгаажуулж бодит хүнтэй холбоно; тэр үед
-- google_sub нь тухайн eID хэрэглэгчийн мөрд бичигдэнэ. Дараа нь google_sub-аар
-- шууд нэвтэрнэ. Багана nullable тул нууц үг/eID-only хэрэглэгчид эвдрэхгүй.

ALTER TABLE users ADD COLUMN IF NOT EXISTS google_sub TEXT;

-- Нэг Google account зөвхөн нэг (soft-delete хийгдээгүй) хэрэглэгчид холбогдоно.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_google_sub_active
    ON users (google_sub)
    WHERE google_sub IS NOT NULL AND deleted_at IS NULL;
