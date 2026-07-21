-- Government Template Platform V3.0
-- Platform-хоорондын үйлчилгээний хүсэлт дамжуулах + SLA хяналт. Дээд platform-оос
-- хугацаатай хүсэлт хүлээж авч, доод platform-ууд руу дамжуулж (routing rules),
-- заагдсан хугацаанд биелэлтийг хянаж/шахаж, хариуг цуглуулна. Эдгээр нь
-- platform-хоорондын тохиргоо/telemetry (per-citizen биш) тул gateway-ийн адил
-- RLS-гүй (roles/permissions-тэй ижил ангилал).

-- Доод (эсвэл дээд) platform-ын бүртгэл.
CREATE TABLE IF NOT EXISTS relay_platforms (
    id                 uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    code               TEXT UNIQUE NOT NULL,
    name               TEXT NOT NULL,
    endpoint_url       TEXT NOT NULL DEFAULT '',
    supervisor_contact TEXT NOT NULL DEFAULT '',
    enabled            BOOLEAN NOT NULL DEFAULT true,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Чиглүүлэлтийн дүрэм: service_code → platform (target бүрийн SLA минутаар).
CREATE TABLE IF NOT EXISTS relay_routes (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    service_code TEXT NOT NULL,
    platform_id  uuid NOT NULL REFERENCES relay_platforms(id) ON DELETE CASCADE,
    sla_minutes  INT  NOT NULL DEFAULT 60,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (service_code, platform_id)
);
CREATE INDEX IF NOT EXISTS idx_relay_routes_service ON relay_routes (service_code);

-- Дээд platform-оос ирсэн хугацаатай хүсэлт.
CREATE TABLE IF NOT EXISTS relay_requests (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_platform TEXT NOT NULL DEFAULT '',
    external_ref    TEXT NOT NULL DEFAULT '',
    service_code    TEXT NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    payload         JSONB NOT NULL DEFAULT '{}'::jsonb,
    priority        TEXT NOT NULL DEFAULT 'normal',
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    due_at          TIMESTAMPTZ NOT NULL,
    status          TEXT NOT NULL DEFAULT 'received',
    result          JSONB,
    fulfilled_at    TIMESTAMPTZ,
    breach_notified BOOLEAN NOT NULL DEFAULT false,
    updated_at      TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_relay_requests_status_due ON relay_requests (status, due_at);
CREATE INDEX IF NOT EXISTS idx_relay_requests_received ON relay_requests (received_at DESC);

-- Downstream platform тус бүрд оногдсон дэд даалгавар.
CREATE TABLE IF NOT EXISTS relay_assignments (
    id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id     uuid NOT NULL REFERENCES relay_requests(id) ON DELETE CASCADE,
    platform_id    uuid NOT NULL REFERENCES relay_platforms(id) ON DELETE CASCADE,
    status         TEXT NOT NULL DEFAULT 'pending',
    due_at         TIMESTAMPTZ NOT NULL,
    dispatched_at  TIMESTAMPTZ,
    responded_at   TIMESTAMPTZ,
    result         JSONB,
    reminders_sent INT NOT NULL DEFAULT 0,
    escalated      BOOLEAN NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS idx_relay_assignments_status_due ON relay_assignments (status, due_at);
CREATE INDEX IF NOT EXISTS idx_relay_assignments_request ON relay_assignments (request_id);

-- Timeline / realtime feed.
CREATE TABLE IF NOT EXISTS relay_events (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id    uuid NOT NULL REFERENCES relay_requests(id) ON DELETE CASCADE,
    assignment_id uuid REFERENCES relay_assignments(id) ON DELETE CASCADE,
    type          TEXT NOT NULL,
    detail        TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_relay_events_created ON relay_events (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_relay_events_request ON relay_events (request_id);

-- Permission-ууд (domain.PermRelayView / PermRelayManage-тэй тохирно). 'admin'
-- бүх каталогид авто-resolve хийдэг тул role_permissions мөр шаардлагагүй.
INSERT INTO permissions(key, label, category) VALUES
    ('relay.view', 'SLA хяналтын самбар үзэх', 'administration'),
    ('relay.manage', 'Хүсэлт дамжуулах чиглүүлэлт удирдах', 'administration')
ON CONFLICT (key) DO NOTHING;

-- ── Demo seed (хоосон үед) — доод platform-ууд + чиглүүлэлт. RELAY_DEMO_MODE
-- идэвхтэй үед simulator эдгээрийн нэрийн өмнөөс хариу үүсгэж, dashboard-ыг
-- өөрөө хөдөлгөнө.
INSERT INTO relay_platforms (code, name, endpoint_url, supervisor_contact)
SELECT * FROM (VALUES
    ('khoroo',   'Хороо (эргэн тойрны захиргаа)', 'demo://loopback', 'supervisor.khoroo@dgov.mn'),
    ('police',   'Цагдаагийн ерөнхий газар',       'demo://loopback', 'supervisor.police@dgov.mn'),
    ('tax',      'Татварын ерөнхий газар',          'demo://loopback', 'supervisor.tax@dgov.mn'),
    ('civil-reg','Улсын бүртгэлийн ерөнхий газар',  'demo://loopback', 'supervisor.civreg@dgov.mn')
) AS v(code, name, endpoint_url, supervisor_contact)
WHERE NOT EXISTS (SELECT 1 FROM relay_platforms);

-- Чиглүүлэлт: нэг үйлчилгээ хэд хэдэн доод platform-д хуваарилагдана.
INSERT INTO relay_routes (service_code, platform_id, sla_minutes)
SELECT v.service_code, p.id, v.sla_minutes
FROM (VALUES
    ('residence-cert', 'khoroo',    30),
    ('residence-cert', 'civil-reg', 45),
    ('background-check','police',    60),
    ('tax-clearance',  'tax',       40)
) AS v(service_code, platform_code, sla_minutes)
JOIN relay_platforms p ON p.code = v.platform_code
WHERE NOT EXISTS (SELECT 1 FROM relay_routes)
ON CONFLICT DO NOTHING;
