-- Government Template Platform V3.0
-- 17_org_rls_recursion_fix-ийг буцаана — migration 14-ийн (recursive) policy-д эргүүлнэ.

DROP POLICY IF EXISTS organizations_member ON organizations;
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

DROP POLICY IF EXISTS org_memberships_member_select ON organization_memberships;
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

DROP FUNCTION IF EXISTS app_is_org_member(uuid, uuid);
