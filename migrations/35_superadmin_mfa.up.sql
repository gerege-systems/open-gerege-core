-- Government Template Platform V3.0
-- Superadmin-ий MFA onboarding: email баталгаажуулалт, TOTP 2FA, recovery code,
-- болон superadmin урилга. TOTP secret нь AES-GCM-ээр шифрлэгдэж хадгалагдана
-- (usecase давхаргад); recovery code нь SHA-256 hash, нэг удаагийн.

-- users: MFA талбарууд.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS mfa_enabled    BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS totp_secret    TEXT;  -- AES-GCM ciphertext (nullable)

-- Recovery code — нэг удаагийн, hash хийж хадгална. Per-user тул RLS.
CREATE TABLE IF NOT EXISTS user_recovery_codes (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  text NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_user_recovery_codes_user ON user_recovery_codes (user_id);

ALTER TABLE user_recovery_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_recovery_codes FORCE ROW LEVEL SECURITY;
CREATE POLICY urc_service ON user_recovery_codes
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');
CREATE POLICY urc_admin ON user_recovery_codes
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');
CREATE POLICY urc_self ON user_recovery_codes
    USING (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    );

-- Superadmin урилга — зөвшөөрөгдсөн email-ийн allow-list. Нийтийн config тул
-- RLS-гүй; app бүрэн CRUD хийдэг (үүсгэх/жагсаах/устгах/accepted тэмдэглэх).
CREATE TABLE IF NOT EXISTS superadmin_invites (
    email       text PRIMARY KEY,
    invited_by  text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    accepted_at timestamptz
);
