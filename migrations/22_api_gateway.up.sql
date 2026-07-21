-- Government Template Platform V3.0
-- API Gateway (нэгдсэн, эцсийн хэлбэр): upstream service-үүд + нэгдсэн
-- 'applications' бүртгэл (gateway consumer + SSO RP-г НЭГ загварт нэгтгэсэн) +
-- request-log telemetry. Application бүр = Hydra OAuth2 client; аппад зөвшөөрсөн
-- service-үүдийг application_services-ээр (OAuth scope) оноодог. Эдгээр нь
-- gateway CONFIG/telemetry хүснэгтүүд — per-user өгөгдөл БИШ — тул RLS-гүй
-- (roles/permissions-тэй ижил ангилал); app бүрэн CRUD хийдэг.

-- Upstream backend service-үүд. scope нь аппад олгох OAuth scope нэр.
CREATE TABLE IF NOT EXISTS gateway_services (
    id                 uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    name               TEXT UNIQUE NOT NULL,
    protocol           TEXT NOT NULL DEFAULT 'https',
    host               TEXT NOT NULL,
    port               INT  NOT NULL DEFAULT 443,
    path               TEXT NOT NULL DEFAULT '/',
    retries            INT  NOT NULL DEFAULT 3,
    connect_timeout_ms INT  NOT NULL DEFAULT 60000,
    tags               TEXT[] NOT NULL DEFAULT '{}',
    enabled            BOOLEAN NOT NULL DEFAULT true,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ,
    scope              TEXT NOT NULL DEFAULT ''
);

-- Нэгдсэн бүртгэл: gateway consumer + SSO RP (developer_apps) → applications.
-- Application бүр = Hydra OAuth2 client (RP = authorization_code, m2m =
-- client_credentials). redirect_uris нь Hydra-гийн толин тусгал (display).
CREATE TABLE IF NOT EXISTS applications (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id     text UNIQUE NOT NULL,           -- Hydra OAuth2 client_id
    name          text NOT NULL,
    app_type      text NOT NULL DEFAULT 'm2m',    -- web | spa | native | m2m
    tags          text[] NOT NULL DEFAULT '{}',
    redirect_uris text[] NOT NULL DEFAULT '{}',
    enabled       boolean NOT NULL DEFAULT true,
    created_by    text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz
);

-- Аппад зөвшөөрсөн service-үүд (байгаа мөр = зөвшөөрөгдсөн). App-ийн Hydra
-- client-ийн scope нь эдгээр service-ийн scope-уудаас бүрдэнэ.
CREATE TABLE IF NOT EXISTS application_services (
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    service_id     uuid NOT NULL REFERENCES gateway_services(id) ON DELETE CASCADE,
    PRIMARY KEY (application_id, service_id)
);
CREATE INDEX IF NOT EXISTS idx_application_services_service ON application_services (service_id);

-- Request log / telemetry — middleware нь бодит /api хүсэлтүүдийг (method/path/
-- status/latency/ip) бичдэг. Route/consumer холбоосгүй (тэдгээр gateway concept
-- хасагдсан).
CREATE TABLE IF NOT EXISTS gateway_request_logs (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    method      TEXT NOT NULL,
    path        TEXT NOT NULL,
    status      INT  NOT NULL,
    latency_ms  INT  NOT NULL DEFAULT 0,
    client_ip   TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_gateway_request_logs_created ON gateway_request_logs (created_at DESC);

-- API Gateway admin surface-ийн permission (domain.PermGatewayManage-тэй
-- тохирно). 'admin' нь бүх каталогид авто-resolve хийдэг тул role_permissions
-- мөр шаардлагагүй.
INSERT INTO permissions(key, label, category) VALUES
    ('gateway.manage', 'API Gateway удирдах', 'administration')
ON CONFLICT (key) DO NOTHING;

-- ── Бодит seed (хоосон үед) — DAN-ий гуравдагч талд өгдөг service/RP-үүд.
-- Тэмдэглэл: SQL нь Hydra client үүсгэхгүй; OAuth-ыг бүрэн идэвхжүүлэхийн тулд
-- админ UI-аас secret эргүүлж/дахин үүсгэнэ.
INSERT INTO gateway_services (name, protocol, host, port, path, tags, scope)
SELECT * FROM (VALUES
    ('dan-sso',  'https', 'sso.dgov.mn', 443, '/oauth2',  ARRAY['sso', 'oidc']::text[], 'svc:dan-sso'),
    ('eid-sign', 'https', 'sso.dgov.mn', 443, '/rp/sign', ARRAY['eid', 'sign']::text[], 'svc:eid-sign')
) AS v(name, protocol, host, port, path, tags, scope)
WHERE NOT EXISTS (SELECT 1 FROM gateway_services);

INSERT INTO applications (client_id, name, app_type, tags, redirect_uris, enabled, created_by)
SELECT * FROM (VALUES
    ('template-dgov-mn',  'template.dgov.mn',  'web', ARRAY['rp']::text[],
        ARRAY['https://template.dgov.mn/auth/callback']::text[], true, 'seed-rp'),
    ('developer-dgov-mn', 'developer.dgov.mn', 'web', ARRAY['rp', 'developer']::text[],
        ARRAY['https://developer.dgov.mn/auth/callback']::text[], true, 'seed-rp')
) AS v(client_id, name, app_type, tags, redirect_uris, enabled, created_by)
WHERE NOT EXISTS (SELECT 1 FROM applications);

-- Бодит RP-үүдэд eID гарын үсэг (eid-sign) service-ийн хандалт олгоно.
INSERT INTO application_services (application_id, service_id)
SELECT a.id, s.id
FROM applications a
JOIN gateway_services s ON s.name = 'eid-sign'
WHERE a.created_by = 'seed-rp'
ON CONFLICT DO NOTHING;
