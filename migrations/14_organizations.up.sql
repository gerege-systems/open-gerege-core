-- Government Template Platform V3.0
-- Organizations + membership: a net-new feature slice. `organizations` holds
-- the legal-entity record (reg_no is case-insensitive unique via a lower()
-- index); `organization_memberships` joins users to orgs with a per-org role
-- (owner/admin/member). Both tables are RLS-protected mirroring the users
-- table: per-request identity arrives as the app.user_id / app.user_role GUCs
-- (SET LOCAL inside the repository's withRLS transaction). Policies are
-- permissive (OR'd); if none matches the query sees zero rows (fail-closed).
-- Write authorization (who may add/remove members or create orgs) is enforced
-- in the usecase layer — RLS here only governs row visibility.

CREATE TABLE IF NOT EXISTS organizations (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    reg_no     TEXT NOT NULL,
    name       TEXT NOT NULL,
    name_latin TEXT NOT NULL DEFAULT '',
    created_by uuid NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

-- Case-insensitive UNIQUE on reg_no over the live (not soft-deleted) rows.
-- lower() makes "1234567" and "1234567" collapse the same way the users
-- table normalizes email; the partial predicate keeps a soft-deleted reg_no
-- from blocking a re-registration.
CREATE UNIQUE INDEX IF NOT EXISTS idx_organizations_reg_no_lower
    ON organizations (lower(reg_no))
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_organizations_created_by ON organizations (created_by);

CREATE TABLE IF NOT EXISTS organization_memberships (
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_org_memberships_user ON organization_memberships (user_id);

-- ---------------------------------------------------------------------------
-- Row-Level Security. RLS is bypassed by Postgres superusers and BYPASSRLS
-- roles; for these policies to enforce, the api must connect as a non-superuser
-- role (see docs/SECURITY.md). FORCE applies RLS even to the table owner.
-- ---------------------------------------------------------------------------

ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE organizations FORCE ROW LEVEL SECURITY;
ALTER TABLE organization_memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE organization_memberships FORCE ROW LEVEL SECURITY;

-- service: trusted pre-auth / system flows. Full access.
CREATE POLICY organizations_service ON organizations
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');

-- admin: full access to every org.
CREATE POLICY organizations_admin ON organizations
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');

-- user: may SELECT only orgs they are a member of. Writes (INSERT/UPDATE) are
-- additionally gated in the usecase by the caller's membership role, so the
-- WITH CHECK here only needs to confirm the row is one the user can see —
-- membership-aware visibility. NULLIF guards an empty GUC from ''::uuid.
CREATE POLICY organizations_member ON organizations
    USING (
        current_setting('app.user_role', true) = 'user'
        AND EXISTS (
            SELECT 1 FROM organization_memberships m
            WHERE m.org_id = organizations.id
              AND m.user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
        )
    )
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND EXISTS (
            SELECT 1 FROM organization_memberships m
            WHERE m.org_id = organizations.id
              AND m.user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
        )
    );

-- service: full access to memberships.
CREATE POLICY org_memberships_service ON organization_memberships
    USING (current_setting('app.user_role', true) = 'service')
    WITH CHECK (current_setting('app.user_role', true) = 'service');

-- admin: full access to every membership.
CREATE POLICY org_memberships_admin ON organization_memberships
    USING (current_setting('app.user_role', true) = 'admin')
    WITH CHECK (current_setting('app.user_role', true) = 'admin');

-- user: may SELECT memberships of orgs they themselves belong to (so a member
-- can list co-members). Writes for a regular user are funnelled through the
-- usecase, which performs add/remove/role-change under the service GUC after
-- checking the caller's own membership role — so no permissive user-write
-- policy is granted here (fail-closed for direct writes).
CREATE POLICY org_memberships_member_select ON organization_memberships
    FOR SELECT
    USING (
        current_setting('app.user_role', true) = 'user'
        AND EXISTS (
            SELECT 1 FROM organization_memberships self
            WHERE self.org_id = organization_memberships.org_id
              AND self.user_id = NULLIF(current_setting('app.user_id', true), '')::uuid
        )
    );
