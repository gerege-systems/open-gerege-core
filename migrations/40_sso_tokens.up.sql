-- Government Template Platform V3.0
-- sso_tokens: stores each citizen's dgov-SSO OAuth tokens so the app can call
-- the SSO eID proxy (sso.dgov.mn/api/v1/eid/*) on their behalf. The access
-- token is short-lived; the refresh token (offline_access) renews it. Both are
-- encrypted at rest with INTEGRATION_ENC_KEY (AES-256-GCM) in the repository —
-- the columns only ever hold ciphertext.
--
-- RLS mirrors the users table (service/admin/self). The refresh path runs in
-- the authenticated user's context (RoleUser), so the self policy must permit
-- read + write on the user's own row; login storage runs as RoleService.

CREATE TABLE sso_tokens (
    user_id           uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    access_token_enc  text        NOT NULL,
    refresh_token_enc text        NOT NULL,
    access_expires_at timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE sso_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE sso_tokens FORCE ROW LEVEL SECURITY;

-- service: trusted pre-/system flows (SSO callback stores tokens before the
-- authenticated identity exists on this request). Full access.
CREATE POLICY sso_tokens_service ON sso_tokens
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');

-- admin: full access.
CREATE POLICY sso_tokens_admin ON sso_tokens
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');

-- user: only their own token row (read for proxy calls, write on refresh).
CREATE POLICY sso_tokens_self ON sso_tokens
    USING (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    )
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
    );
