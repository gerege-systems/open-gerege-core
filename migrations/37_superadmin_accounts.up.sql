-- Government Template Platform V3.0
-- Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.
--
-- Super admin-ы бүртгэлийг ТУСДАА хүснэгтэд (satellite) гаргана.
--
-- Super admin нь users-д role_id=1 мөр хэвээр (auth/RBAC/JWT өөрчлөгдөхгүй), ГЭХДЭЭ
-- google_sub/email-ээр түлхүүрлэгдэнэ (civil_id-г users-д ТАВИХГҮЙ) — ингэснээр нэг
-- хүн eID-ээр admin (civil_id-тэй мөр), Google-оор super admin (тусдаа мөр) байж
-- чадна, civil_id-ийн partial unique index (migration 13) зөрчихгүй. eID баталгаа
-- (civil_id/national_id), TOTP/MFA/email баталгаажуулалт, урилга/onboarding
-- metadata нь энэ хүснэгтэд хадгалагдана.

CREATE TABLE IF NOT EXISTS superadmin_accounts (
    user_id        uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- eID баталгаа (users.civil_id-д ТАВИХГҮЙ; зөвхөн эндээс лавлана).
    civil_id       text,
    national_id    text,
    -- MFA — TOTP secret нь AES-GCM ciphertext (usecase давхаргад шифрлэгдсэн).
    email_verified boolean NOT NULL DEFAULT false,
    mfa_enabled    boolean NOT NULL DEFAULT false,
    totp_secret    text,
    -- Onboarding metadata.
    invited_by     text NOT NULL DEFAULT '',
    onboarded_at   timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz
);
CREATE INDEX IF NOT EXISTS idx_superadmin_accounts_civil ON superadmin_accounts (lower(civil_id)) WHERE civil_id IS NOT NULL;

-- RLS — super admin-ы credential нь маш эмзэг тул service (onboarding/login) болон
-- admin давхаргаас л хандана (user_recovery_codes-ийн загвартай ижил).
ALTER TABLE superadmin_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE superadmin_accounts FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS sa_acct_service ON superadmin_accounts;
CREATE POLICY sa_acct_service ON superadmin_accounts
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');
DROP POLICY IF EXISTS sa_acct_admin ON superadmin_accounts;
CREATE POLICY sa_acct_admin ON superadmin_accounts
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');

-- MFA/email-verified багануудыг users-ээс хасна — эдгээр нь зөвхөн super admin-д
-- хэрэглэгддэг байсан бөгөөд одоо superadmin_accounts-д шилжсэн. (Одоогоор super
-- admin 0 тул өгөгдөл алдагдахгүй.)
ALTER TABLE users DROP COLUMN IF EXISTS mfa_enabled;
ALTER TABLE users DROP COLUMN IF EXISTS totp_secret;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
