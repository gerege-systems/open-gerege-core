-- eID based AI enabled Government Template Platform V3.0
-- sso.dgov.mn-ийг OIDC provider (Ory Hydra + ssod-style login/consent) болгох
-- persistence: RP апп бүртгэл, /admin API key-үүд, нэвтрэлтийн audit. Эдгээр нь
-- глобал/оператор гадаргуу тул RLS-гүй (gov_services-тэй адил лавлах загвар) —
-- эрхийг role-ийн table privilege (initdb-ийн default grant) хамгаална.

-- developer_apps: RP (OAuth2 client) бүртгэл, иргэн бүрийн эзэмшилтэй. Row нь
-- Hydra client_id-г эзэмшигч иргэний eid_sub-тай холбоно (Hydra-д "owner" гэдэг
-- ойлголт байхгүй тул энэ row нь эрх мэдлийн эх сурвалж — устгахдаа Hydra client-
-- тэй хамт устгана).
CREATE TABLE IF NOT EXISTS developer_apps (
    client_id     text PRIMARY KEY,
    owner_eid_sub text NOT NULL,
    name          text NOT NULL,
    redirect_uris text[] NOT NULL DEFAULT '{}',
    scopes        text[] NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS developer_apps_owner_idx ON developer_apps (owner_eid_sub);

-- admin_api_keys: /admin гадаргууг баталгаажуулах SHA-256-hash хийсэн key-үүд.
-- Env-ээр өгсөн bootstrap key-үүд ЭНД хадгалагдахгүй (санах ойд байна).
CREATE TABLE IF NOT EXISTS admin_api_keys (
    id           text PRIMARY KEY,
    name         text NOT NULL,
    hash         text NOT NULL UNIQUE,
    display      text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    disabled     boolean NOT NULL DEFAULT false
);

-- login_events: eID / Google нэвтрэлт бүрийн append-only audit; upstream payload
-- бүрэн jsonb-ээр хадгалагдана. is_new_device нь тухайн иргэнд өмнө нь харагдаагүй
-- төхөөрөмж/IP-ээс нэвтэрсэн эсэхийг тэмдэглэнэ (account-takeover дохио).
CREATE TABLE IF NOT EXISTS login_events (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    eid_sub       text,
    method        text NOT NULL,                 -- 'eid' | 'google'
    national_id   text,
    google_sub    text,
    raw_claims    jsonb NOT NULL DEFAULT '{}'::jsonb,
    ip            text,
    user_agent    text,
    is_new_device boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS login_events_eid_sub_idx ON login_events (eid_sub, created_at DESC);
