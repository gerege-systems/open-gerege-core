-- Government Template Platform V3.0
-- Хэрэглэгчийн гуравдагч этгээдийн интеграцийн (Google Drive/Meet, Dropbox)
-- OAuth токеныг хадгална. Токенууд usecase давхаргад шифрлэгдэж ирдэг тул
-- хүснэгтэд шифрлэгдсэн (ciphertext) хэлбэрээр хадгалагдана. Хэрэглэгч-тус-бүрийн
-- мэдрэмтгий өгөгдөл тул users-тэй адил Row-Level Security-гээр хамгаална
-- (migration 7 / 20-ийн загвар). Хүснэгтэд DML эрх нь initdb-ийн ALTER DEFAULT
-- PRIVILEGES-ээр (migrate = superuser үүсгэсэн бүх шинэ хүснэгтэд app role-д
-- авто олгогдоно) ирнэ.

CREATE TABLE IF NOT EXISTS user_integrations (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider      TEXT NOT NULL,
    access_token  TEXT NOT NULL,
    refresh_token TEXT,
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_user_integrations_user ON user_integrations(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_integrations_user_provider
    ON user_integrations(user_id, provider);

-- RLS: users хүснэгттэй ижил бодлогын загвар (service/admin/self).
ALTER TABLE user_integrations ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_integrations FORCE ROW LEVEL SECURITY;

CREATE POLICY user_integrations_service ON user_integrations
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');

CREATE POLICY user_integrations_admin ON user_integrations
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');

CREATE POLICY user_integrations_self ON user_integrations
    USING (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    );
