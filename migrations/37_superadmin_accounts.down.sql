-- Government Template Platform V3.0
-- Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.
--
-- 37_superadmin_accounts-ийн буцаалт. MFA багануудыг users-д эргүүлэн нэмж
-- (migration 35-ийн адил), satellite хүснэгтийг устгана. АНХААР: superadmin_accounts
-- дахь өгөгдөл (TOTP secret зэрэг) буцаах үед устна.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS mfa_enabled    BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS totp_secret    TEXT;

DROP TABLE IF EXISTS superadmin_accounts;
