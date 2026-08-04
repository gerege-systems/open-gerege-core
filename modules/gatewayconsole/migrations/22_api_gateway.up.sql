-- API Gateway — upstream service-үүд + request-log telemetry.
--
-- ХУВААЛТ: анх энэ файл `applications` ба `application_services`-ыг ч
-- үүсгэдэг байсан. Тэдгээр нь applications модулийн өмч тул
-- modules/applications/migrations/22_applications.up.sql руу нүүсэн.
-- application_services нь gateway_services рүү FK-тэй учир applications
-- модуль ЭНЭ модулийн ДАРАА ажиллана (modules/sources.go-г үз).
--
-- Эдгээр нь gateway CONFIG/telemetry хүснэгтүүд — per-user өгөгдөл БИШ —
-- тул RLS-гүй (roles/permissions-тэй ижил ангилал).

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

-- Request log / telemetry — middleware нь бодит /api хүсэлтүүдийг (method/path/
-- status/latency/ip) бичдэг.
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

-- ── Бодит seed (хоосон үед) — DAN-ий гуравдагч талд өгдөг service-үүд.
INSERT INTO gateway_services (name, protocol, host, port, path, tags, scope)
SELECT * FROM (VALUES
    ('dan-sso',  'https', 'sso.gerege.mn', 443, '/oauth2',  ARRAY['sso', 'oidc']::text[], 'svc:dan-sso'),
    ('eid-sign', 'https', 'sso.gerege.mn', 443, '/rp/sign', ARRAY['eid', 'sign']::text[], 'svc:eid-sign')
) AS v(name, protocol, host, port, path, tags, scope)
WHERE NOT EXISTS (SELECT 1 FROM gateway_services);
