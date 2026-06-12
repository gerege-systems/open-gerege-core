-- Gerege Template Version 27.0
-- "Login with eID" — энэ template нь Relying Party. eID хэрэглэгчид нууц
-- үггүй, email-гүй; давтагдашгүй байдлыг national_id-ээр хангана. Бүх багана
-- nullable / хоосон default тул одоо байгаа мөрүүд эвдрэхгүй (backward compatible).

-- eID identity баганууд.
ALTER TABLE users ADD COLUMN IF NOT EXISTS national_id TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS civil_id    TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS kyc_level   TEXT;

-- eID хэрэглэгч нууц үггүй тул password багана NULL байж болохоор болгоно
-- (migration 1-д NOT NULL байсан). Нууц үгтэй хэрэглэгчид өмнөх адил.
ALTER TABLE users ALTER COLUMN password DROP NOT NULL;

-- eID хэрэглэгч email-гүй тул email багана NULL байж болохоор болгоно
-- (migration 1-д NOT NULL байсан). migration 3 нь email-ийн UNIQUE
-- constraint-ийг partial index болгож сольсон; тэр индексийг олон NULL/хоосон
-- email мөргөлдөхгүйгээр зэрэгцэн орших боломжтой болгож шинэчилнэ.
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

DROP INDEX IF EXISTS idx_users_email_active;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active
    ON users(email)
    WHERE deleted_at IS NULL AND email IS NOT NULL AND email <> '';

-- national_id нь eID хэрэглэгчийн давтагдашгүй түлхүүр — жижиг үсгээр,
-- зөвхөн утгатай (NULL биш) мөрүүд дээр давтагдашгүй.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_national_id_active
    ON users(lower(national_id))
    WHERE national_id IS NOT NULL;
