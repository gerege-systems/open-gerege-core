-- Нэгдсэн апп бүртгэл: gateway consumer + SSO RP (developer_apps) →
-- applications. Application бүр = Hydra OAuth2 client.
--
-- ХУВААЛТ: анх 22_api_gateway.up.sql дотор байсан. application_services нь
-- gateway_services рүү FK-тэй тул энэ модуль gateway-console-ийн ДАРАА
-- ажиллана (modules/sources.go).
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

-- Аппад зөвшөөрсөн service-үүд (байгаа мөр = зөвшөөрөгдсөн).
CREATE TABLE IF NOT EXISTS application_services (
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    service_id     uuid NOT NULL REFERENCES gateway_services(id) ON DELETE CASCADE,
    PRIMARY KEY (application_id, service_id)
);
CREATE INDEX IF NOT EXISTS idx_application_services_service ON application_services (service_id);

-- ── Бодит seed (хоосон үед) — RP-үүд.
INSERT INTO applications (client_id, name, app_type, tags, redirect_uris, enabled, created_by)
SELECT * FROM (VALUES
    ('template-dgov-mn',  'template.gerege.mn',  'web', ARRAY['rp']::text[],
        ARRAY['https://template.gerege.mn/auth/callback']::text[], true, 'seed-rp'),
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
