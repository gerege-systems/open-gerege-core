-- Gerege Template Version 27.0
-- Two net-new append-only event stores: a hash-chained `audit_log` and a
-- RASP-style `security_events` ingest table. Both are RLS-protected. The audit
-- log is admin/service-only end-to-end (regular users may neither read nor
-- write it); security_events lets any authenticated user INSERT their own row
-- but never read. Writers set the app.user_id / app.user_role GUCs with
-- SET LOCAL inside the repository transaction (mirroring users/org). FORCE
-- applies RLS even to the table owner; for the policies to actually enforce
-- the api must connect as a non-superuser role (see docs/SECURITY.md).

-- ===========================================================================
-- SECTION 1 — Hash-chained, append-only audit log.
--
-- Each row's chain_hash = SHA-256(hex(prev_hash) || canonical-json(row-without-hash)).
-- A single global chain (NOT sharded — the eID reference shards by user_id%16
-- for throughput; the template keeps one chain for simplicity and easy
-- verification). VerifyChain walks rows from genesis recomputing each hash.
-- ===========================================================================

CREATE TABLE IF NOT EXISTS audit_log (
    id            BIGSERIAL PRIMARY KEY,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_user_id uuid,
    action        TEXT NOT NULL,
    category      TEXT,
    target        TEXT,
    request_id    TEXT,
    metadata      JSONB,
    prev_hash     TEXT,
    chain_hash    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_log_occurred_at ON audit_log (occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log (actor_user_id);

ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log FORCE ROW LEVEL SECURITY;

-- service: trusted system writer. The audit repository runs Append under this
-- GUC so it can log an event on behalf of ANY actor (including unauthenticated
-- system flows) regardless of the request's user identity.
CREATE POLICY audit_log_service ON audit_log
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');

-- admin: full read/insert. Admin read API runs under this GUC.
CREATE POLICY audit_log_admin ON audit_log
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');

-- No policy for the 'user' role: regular users see zero rows and cannot insert
-- (fail-closed). Audit is admin-only.

-- ===========================================================================
-- SECTION 2 — RASP-style security events ingest.
--
-- Clients/runtime post security signals (jailbreak, integrity, anomalies).
-- Any authenticated user may INSERT a row for THEMSELVES (user_id = app.user_id)
-- but cannot read; admin/service have full access.
-- ===========================================================================

CREATE TABLE IF NOT EXISTS security_events (
    id          BIGSERIAL PRIMARY KEY,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id     uuid,
    kind        TEXT NOT NULL,
    severity    TEXT,
    source      TEXT,
    user_agent  TEXT,
    ip          TEXT,
    detail      JSONB
);

CREATE INDEX IF NOT EXISTS idx_security_events_received_at ON security_events (received_at);
CREATE INDEX IF NOT EXISTS idx_security_events_user ON security_events (user_id);

ALTER TABLE security_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE security_events FORCE ROW LEVEL SECURITY;

-- service: full access (system ingest / background processing).
CREATE POLICY security_events_service ON security_events
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');

-- admin: full read/insert (admin list API).
CREATE POLICY security_events_admin ON security_events
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');

-- user: may INSERT only a row about themselves (user_id must equal their own
-- app.user_id) and may NOT read (no USING clause grants SELECT). NULLIF guards
-- an empty GUC from ''::uuid.
CREATE POLICY security_events_user_insert ON security_events
    FOR INSERT
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    );
