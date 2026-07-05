-- Gerege Template Version 27.0
-- API Gateway: upstream services, routes, consumers + API keys, per-route
-- policies (rate-limit / auth / cors …) and a request-log table for the admin
-- "API Gateway" system. These are gateway CONFIG/telemetry tables — not
-- per-user data — so they are NOT under RLS (same class as roles/permissions).

-- Upstream backend services that routes proxy to.
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
    updated_at         TIMESTAMPTZ
);

-- Routes: which (methods, paths) map to which upstream service.
CREATE TABLE IF NOT EXISTS gateway_routes (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    service_id    uuid NOT NULL REFERENCES gateway_services(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    methods       TEXT[] NOT NULL DEFAULT '{GET}',
    paths         TEXT[] NOT NULL DEFAULT '{}',
    strip_path    BOOLEAN NOT NULL DEFAULT true,
    preserve_host BOOLEAN NOT NULL DEFAULT false,
    enabled       BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_gateway_routes_service ON gateway_routes (service_id);

-- Consumers: registered API clients.
CREATE TABLE IF NOT EXISTS gateway_consumers (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    username   TEXT UNIQUE NOT NULL,
    custom_id  TEXT NOT NULL DEFAULT '',
    tags       TEXT[] NOT NULL DEFAULT '{}',
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

-- API keys belonging to a consumer. Only the SHA-256 hash is stored; the
-- prefix is kept for display ("gk_live_ab12…"). The plaintext key is shown
-- to the operator exactly once at creation time.
CREATE TABLE IF NOT EXISTS gateway_api_keys (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    consumer_id  uuid NOT NULL REFERENCES gateway_consumers(id) ON DELETE CASCADE,
    label        TEXT NOT NULL DEFAULT '',
    key_prefix   TEXT NOT NULL,
    key_hash     TEXT NOT NULL,
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    revoked      BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_gateway_api_keys_consumer ON gateway_api_keys (consumer_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_gateway_api_keys_hash ON gateway_api_keys (key_hash);

-- Policies (plugins): rate-limit, key-auth, cors, … applied to a route, or
-- global when route_id IS NULL. config is plugin-specific JSON.
CREATE TABLE IF NOT EXISTS gateway_policies (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_id   uuid REFERENCES gateway_routes(id) ON DELETE CASCADE,
    type       TEXT NOT NULL,
    config     JSONB NOT NULL DEFAULT '{}',
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_gateway_policies_route ON gateway_policies (route_id);

-- Request log / telemetry feeding the overview dashboard.
CREATE TABLE IF NOT EXISTS gateway_request_logs (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_id    uuid REFERENCES gateway_routes(id) ON DELETE SET NULL,
    consumer_id uuid REFERENCES gateway_consumers(id) ON DELETE SET NULL,
    method      TEXT NOT NULL,
    path        TEXT NOT NULL,
    status      INT  NOT NULL,
    latency_ms  INT  NOT NULL DEFAULT 0,
    client_ip   TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_gateway_request_logs_created ON gateway_request_logs (created_at DESC);

-- New permission for the API Gateway admin surface (matches
-- domain.PermGatewayManage). 'admin' auto-resolves to the full catalogue, so
-- no explicit role_permissions row is needed for it.
INSERT INTO permissions(key, label, category) VALUES
    ('gateway.manage', 'API Gateway удирдах', 'administration')
ON CONFLICT (key) DO NOTHING;

-- ── Demo seed (only when empty) so the admin screens render meaningfully.
INSERT INTO gateway_services(name, protocol, host, port, path, tags)
SELECT * FROM (VALUES
    ('eid-core',   'https', 'api.eidgerege.mn', 443, '/gerege/v1', ARRAY['eid','core']),
    ('payments',   'https', 'pay.gerege.mn',    443, '/v2',         ARRAY['billing']),
    ('ai-gateway', 'https', 'ai.gerege.mn',     443, '/v1',         ARRAY['ai'])
) AS v(name, protocol, host, port, path, tags)
WHERE NOT EXISTS (SELECT 1 FROM gateway_services);

INSERT INTO gateway_routes(service_id, name, methods, paths, strip_path)
SELECT s.id, r.name, r.methods, r.paths, true
FROM (VALUES
    ('eid-core',   'eid-me',       ARRAY['GET'],         ARRAY['/eid/me']),
    ('eid-core',   'eid-sign',     ARRAY['POST'],        ARRAY['/eid/sign']),
    ('payments',   'pay-charge',   ARRAY['POST'],        ARRAY['/pay/charge']),
    ('ai-gateway', 'ai-chat',      ARRAY['POST','GET'],  ARRAY['/ai/chat'])
) AS r(svc, name, methods, paths)
JOIN gateway_services s ON s.name = r.svc
WHERE NOT EXISTS (SELECT 1 FROM gateway_routes);

INSERT INTO gateway_consumers(username, custom_id, tags)
SELECT * FROM (VALUES
    ('mobile-app', 'app-001', ARRAY['internal']),
    ('partner-bank', 'bank-77', ARRAY['partner']),
    ('analytics-job', 'job-12', ARRAY['internal','batch'])
) AS v(username, custom_id, tags)
WHERE NOT EXISTS (SELECT 1 FROM gateway_consumers);

-- Demo keys: hashes are sha-256 of throwaway demo strings (display prefix only).
INSERT INTO gateway_api_keys(consumer_id, label, key_prefix, key_hash)
SELECT c.id, k.label, k.prefix, k.hash
FROM (VALUES
    ('mobile-app',    'production',  'gk_live_a1b2', encode(sha256('demo-mobile-app'::bytea), 'hex')),
    ('partner-bank',  'production',  'gk_live_c3d4', encode(sha256('demo-partner-bank'::bytea), 'hex')),
    ('analytics-job', 'read-only',   'gk_live_e5f6', encode(sha256('demo-analytics-job'::bytea), 'hex'))
) AS k(username, label, prefix, hash)
JOIN gateway_consumers c ON c.username = k.username
WHERE NOT EXISTS (SELECT 1 FROM gateway_api_keys);

INSERT INTO gateway_policies(route_id, type, config)
SELECT r.id, p.type, p.config::jsonb
FROM (VALUES
    ('eid-me',     'rate-limit', '{"limit":60,"window":"minute"}'),
    ('eid-sign',   'key-auth',   '{"key_in":"header","header_name":"x-api-key"}'),
    ('pay-charge', 'rate-limit', '{"limit":30,"window":"minute"}'),
    ('ai-chat',    'cors',       '{"origins":["https://web.gerege.mn"],"methods":["GET","POST"]}')
) AS p(route, type, config)
JOIN gateway_routes r ON r.name = p.route
WHERE NOT EXISTS (SELECT 1 FROM gateway_policies);

INSERT INTO gateway_request_logs(route_id, consumer_id, method, path, status, latency_ms, client_ip, created_at)
SELECT r.id, c.id, l.method, l.path, l.status, l.latency, l.ip, now() - (l.mins || ' minutes')::interval
FROM (VALUES
    ('eid-me',     'mobile-app',    'GET',  '/eid/me',      200, 42,  '202.131.0.10', 2),
    ('eid-sign',   'mobile-app',    'POST', '/eid/sign',    200, 318, '202.131.0.10', 5),
    ('pay-charge', 'partner-bank',  'POST', '/pay/charge',  201, 121, '203.91.4.7',   9),
    ('ai-chat',    'mobile-app',    'POST', '/ai/chat',     200, 880, '202.131.0.10', 12),
    ('pay-charge', 'partner-bank',  'POST', '/pay/charge',  402, 64,  '203.91.4.7',   17),
    ('eid-me',     'analytics-job', 'GET',  '/eid/me',      429, 8,   '10.0.0.4',     21),
    ('ai-chat',    'partner-bank',  'POST', '/ai/chat',     500, 1503,'203.91.4.7',   33)
) AS l(route, consumer, method, path, status, latency, ip, mins)
JOIN gateway_routes r ON r.name = l.route
JOIN gateway_consumers c ON c.username = l.consumer
WHERE NOT EXISTS (SELECT 1 FROM gateway_request_logs);
