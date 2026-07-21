-- Government Template Platform V3.0
-- Row-Level Security on the users table: defense-in-depth on top of the
-- deleted_at / WHERE clauses the repository already writes. Per-request identity
-- arrives as the app.user_id / app.user_role GUCs, set with SET LOCAL inside the
-- repository's withRLS transaction. Policies are permissive (OR'd together); if
-- no policy matches, the query sees zero rows (fail-closed).
--
-- NOTE: RLS is bypassed by Postgres superusers and BYPASSRLS roles. For these
-- policies to actually enforce, the application must connect as a non-superuser
-- role (see docs/SECURITY.md — DB role separation). FORCE makes RLS apply even
-- to the table owner.

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

-- service: trusted pre-auth / system flows (login email lookup, register
-- INSERT, OTP activation, password reset, seeding). Full access — these run
-- before any authenticated identity exists.
CREATE POLICY users_service ON users
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');

-- admin: full access to every row.
CREATE POLICY users_admin ON users
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');

-- user: only their own row. NULLIF guards against an empty GUC being cast to
-- uuid (''::uuid would raise); an empty id becomes NULL, so `id = NULL` excludes
-- the row instead of erroring.
CREATE POLICY users_self ON users
    USING (
        current_setting('app.user_role', true) = 'user'
        AND id = NULLIF(current_setting('app.user_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND id = NULLIF(current_setting('app.user_id', true), '')::uuid
    );
