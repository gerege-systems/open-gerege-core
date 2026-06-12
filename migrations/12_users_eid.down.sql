-- Gerege Template Version 27.0
-- 12_users_eid-ийг буцаана. eID хэрэглэгч (national_id-тэй, password/email NULL)
-- байхгүй гэж үзнэ — энэ migration-ийг буцаахаас өмнө тэдгээрийг устгасан байх
-- ёстой, эс бөгөөс NOT NULL-ийг сэргээх алхам амжилтгүй болно.

DROP INDEX IF EXISTS idx_users_national_id_active;

-- email-ийн partial unique index-ийг migration 3-ын хэлбэр рүү буцаана.
DROP INDEX IF EXISTS idx_users_email_active;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active
    ON users(email)
    WHERE deleted_at IS NULL;

ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users ALTER COLUMN password SET NOT NULL;

ALTER TABLE users DROP COLUMN IF EXISTS kyc_level;
ALTER TABLE users DROP COLUMN IF EXISTS civil_id;
ALTER TABLE users DROP COLUMN IF EXISTS national_id;
