-- applications / application_services-ийн idempotent нэгтгэл.
--
-- ХУВААЛТ: анх 36_gateway_reconcile.up.sql дотор байсан. Хуучин 22-г
-- бүртгэсэн DB-д эдгээр хүснэгт байхгүй үлдсэн байж болзошгүй тул
-- forward-only reconcile хэвээр (IF NOT EXISTS → нэгтгэсэн DB дээр no-op).
CREATE TABLE IF NOT EXISTS applications (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id     text UNIQUE NOT NULL,
    name          text NOT NULL,
    app_type      text NOT NULL DEFAULT 'm2m',
    tags          text[] NOT NULL DEFAULT '{}',
    redirect_uris text[] NOT NULL DEFAULT '{}',
    enabled       boolean NOT NULL DEFAULT true,
    created_by    text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz
);

CREATE TABLE IF NOT EXISTS application_services (
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    service_id     uuid NOT NULL REFERENCES gateway_services(id) ON DELETE CASCADE,
    PRIMARY KEY (application_id, service_id)
);
CREATE INDEX IF NOT EXISTS idx_application_services_service ON application_services (service_id);
