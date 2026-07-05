-- Gerege Template Version 27.0
-- Иргэний "Төрийн үйлчилгээ" портал (me систем). gov_services нь нийтийн
-- үйлчилгээний каталог (RLS-гүй лавлах); бусад нь хэрэглэгч-тус-бүрийн өгөгдөл
-- бөгөөд user_id-гаар scope хийгдэнэ. Хоёр давхар хамгаалалт: repository query
-- бүр user_id-г ИЛ шүүхээс гадна per-user хүснэгтүүдэд Row-Level Security
-- идэвхжсэн — per-request identity нь app.user_id / app.user_role GUC-ээр
-- (repository-ийн withRLS транзакцид SET LOCAL) ирнэ (migration 7-той адил
-- загвар). Энгийн байлгахын тулд users-тэй FK холбоогүй (users нь RLS-тэй;
-- user_id-г uuid-ээр хадгална).

-- Үйлчилгээний каталог (нийтийн).
CREATE TABLE IF NOT EXISTS gov_services (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    code            TEXT UNIQUE NOT NULL,
    name            TEXT NOT NULL,
    category        TEXT NOT NULL DEFAULT '',
    agency          TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    fee             INT  NOT NULL DEFAULT 0,    -- MNT
    processing_days INT  NOT NULL DEFAULT 0,
    online          BOOLEAN NOT NULL DEFAULT true,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Иргэний хүсэлт (per-user).
CREATE TABLE IF NOT EXISTS gov_applications (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      uuid NOT NULL,
    service_id   uuid REFERENCES gov_services(id) ON DELETE SET NULL,
    service_name TEXT NOT NULL DEFAULT '',
    reference_no TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'submitted', -- submitted/in_review/approved/rejected/completed/cancelled
    note         TEXT NOT NULL DEFAULT '',
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_gov_applications_user ON gov_applications (user_id, submitted_at DESC);

-- Лавлагаа / тодорхойлолт (per-user, олгогдсон баримт).
CREATE TABLE IF NOT EXISTS gov_references (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      uuid NOT NULL,
    type         TEXT NOT NULL,
    title        TEXT NOT NULL,
    reference_no TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'issued',   -- issued/expired
    issued_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    valid_until  TIMESTAMPTZ,
    data         JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_gov_references_user ON gov_references (user_id, issued_at DESC);

-- Мэдэгдэл (per-user).
CREATE TABLE IF NOT EXISTS gov_notifications (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    uuid NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    category   TEXT NOT NULL DEFAULT 'info',        -- info/success/warning
    read       BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_gov_notifications_user ON gov_notifications (user_id, created_at DESC);

-- Төлбөр (татвар/хураамж/торгууль) (per-user).
CREATE TABLE IF NOT EXISTS gov_payments (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    uuid NOT NULL,
    title      TEXT NOT NULL,
    category   TEXT NOT NULL DEFAULT 'fee',          -- tax/fee/fine
    amount     INT  NOT NULL DEFAULT 0,
    currency   TEXT NOT NULL DEFAULT 'MNT',
    status     TEXT NOT NULL DEFAULT 'pending',      -- pending/paid
    due_date   TIMESTAMPTZ,
    paid_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_gov_payments_user ON gov_payments (user_id, created_at DESC);

-- Цаг захиалга (per-user).
CREATE TABLE IF NOT EXISTS gov_appointments (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      uuid NOT NULL,
    service_id   uuid REFERENCES gov_services(id) ON DELETE SET NULL,
    service_name TEXT NOT NULL DEFAULT '',
    agency       TEXT NOT NULL DEFAULT '',
    location     TEXT NOT NULL DEFAULT '',
    scheduled_at TIMESTAMPTZ NOT NULL,
    status       TEXT NOT NULL DEFAULT 'booked',      -- booked/confirmed/cancelled/completed
    note         TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_gov_appointments_user ON gov_appointments (user_id, scheduled_at);

-- ---------------------------------------------------------------------------
-- Row-Level Security (migration 7 / 14-ийн загвар). gov_services нь нийтийн
-- каталог тул RLS-гүй; бусад 5 хүснэгт нь хэрэглэгч-тус-бүрийн тул RLS-тэй.
-- RLS-г Postgres superuser болон BYPASSRLS role алгасдаг тул эдгээр бодлого
-- хүчинтэй байхын тулд api нь non-superuser role-оор холбогдоно
-- (docs/SECURITY.md). FORCE нь table owner-т ч RLS-г хэрэгжүүлнэ. Бодлогууд
-- permissive (OR); аль нь ч таарахгүй бол query тэг мөр харна (fail-closed).
-- Хүснэгтэд DML эрх нь initdb-ийн ALTER DEFAULT PRIVILEGES-ээр (migrate =
-- superuser үүсгэсэн бүх шинэ хүснэгтэд app role-д авто олгогдоно) ирнэ.
-- ---------------------------------------------------------------------------

ALTER TABLE gov_applications  ENABLE ROW LEVEL SECURITY;
ALTER TABLE gov_applications  FORCE  ROW LEVEL SECURITY;
ALTER TABLE gov_references    ENABLE ROW LEVEL SECURITY;
ALTER TABLE gov_references    FORCE  ROW LEVEL SECURITY;
ALTER TABLE gov_notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE gov_notifications FORCE  ROW LEVEL SECURITY;
ALTER TABLE gov_payments      ENABLE ROW LEVEL SECURITY;
ALTER TABLE gov_payments      FORCE  ROW LEVEL SECURITY;
ALTER TABLE gov_appointments  ENABLE ROW LEVEL SECURITY;
ALTER TABLE gov_appointments  FORCE  ROW LEVEL SECURITY;

-- gov_applications
CREATE POLICY gov_applications_service ON gov_applications
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');
CREATE POLICY gov_applications_admin ON gov_applications
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');
CREATE POLICY gov_applications_self ON gov_applications
    USING (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    );

-- gov_references
CREATE POLICY gov_references_service ON gov_references
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');
CREATE POLICY gov_references_admin ON gov_references
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');
CREATE POLICY gov_references_self ON gov_references
    USING (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    );

-- gov_notifications
CREATE POLICY gov_notifications_service ON gov_notifications
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');
CREATE POLICY gov_notifications_admin ON gov_notifications
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');
CREATE POLICY gov_notifications_self ON gov_notifications
    USING (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    );

-- gov_payments
CREATE POLICY gov_payments_service ON gov_payments
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');
CREATE POLICY gov_payments_admin ON gov_payments
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');
CREATE POLICY gov_payments_self ON gov_payments
    USING (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    );

-- gov_appointments
CREATE POLICY gov_appointments_service ON gov_appointments
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');
CREATE POLICY gov_appointments_admin ON gov_appointments
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');
CREATE POLICY gov_appointments_self ON gov_appointments
    USING (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    );

-- ── Каталог seed (нийтийн үйлчилгээнүүд). Хоосон үед л оруулна.
INSERT INTO gov_services(code, name, category, agency, description, fee, processing_days, online)
SELECT * FROM (VALUES
    ('CIVIL_ID',     'Иргэний үнэмлэх захиалах',         'Бүртгэл',            'УБЕГ',          'Иргэний үнэмлэх шинээр авах, дахин захиалах.',        25000, 7,  true),
    ('RESIDENCE',    'Оршин суугаа газрын лавлагаа',     'Бүртгэл',            'УБЕГ',          'Оршин суугаа хаягийн албан ёсны лавлагаа.',           500,   0,  true),
    ('TAX_CLEAR',    'Татварын тодорхойлолт',            'Татвар',             'ТЕГ',           'Татварын өргүй гэсэн тодорхойлолт.',                  0,     1,  true),
    ('SOCIAL_INS',   'Нийгмийн даатгалын лавлагаа',      'Нийгмийн хамгаалал', 'НДЕГ',          'Шимтгэл төлөлтийн дэлгэрэнгүй лавлагаа.',             0,     0,  true),
    ('DRIVER_LIC',   'Жолооны үнэмлэх сунгах',           'Тээвэр',             'Зам тээвэр',    'Жолоочийн үнэмлэхний хугацаа сунгах.',                35000, 5,  false),
    ('MARRIAGE',     'Гэрлэлтийн гэрчилгээ',             'Бүртгэл',            'УБЕГ',          'Гэрлэлт бүртгүүлэх, гэрчилгээ авах.',                 15000, 3,  false),
    ('HEALTH_INS',   'Эрүүл мэндийн даатгал',            'Эрүүл мэнд',         'ЭМД',           'Эрүүл мэндийн даатгалын төлөв, төлбөр.',              0,     0,  true),
    ('BIZ_REG',      'Аж ахуй нэгж бүртгэх',             'Бизнес',             'УБЕГ',          'ХХК/ХК шинээр бүртгүүлэх.',                           44000, 10, true)
) AS v(code, name, category, agency, description, fee, processing_days, online)
WHERE NOT EXISTS (SELECT 1 FROM gov_services);
