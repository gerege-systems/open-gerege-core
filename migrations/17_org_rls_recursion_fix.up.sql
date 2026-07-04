-- Gerege Template Version 27.0
-- Байгууллагын RLS policy-ийн infinite recursion засвар (SQLSTATE 42P17).
--
-- Migration 14-ийн `org_memberships_member_select` policy нь
-- `organization_memberships`-ыг ӨӨРИЙГ нь subquery хийдэг тул RLS дахин
-- ажиллаж хязгааргүй давтагдана; мөн `organizations_member` policy нь
-- `organization_memberships`-ыг уншиж, тэр policy-г өдөөж recursion үүсгэнэ.
--
-- Засвар: гишүүнчлэлийг шалгах SECURITY DEFINER функц ашиглана. Функц нь
-- эзэмшигчийн (migrate = superuser) эрхээр ажиллаж RLS-ийг тойрдог тул
-- subquery дахин policy өдөөхгүй. Функц зөвхөн (org,user) хос байгаа эсэхийг
-- boolean-оор буцаадаг тул elevated эрхтэй ажиллах нь аюулгүй.

CREATE OR REPLACE FUNCTION app_is_org_member(p_org_id uuid, p_user_id uuid)
RETURNS boolean
LANGUAGE sql
SECURITY DEFINER
STABLE
AS $$
    SELECT p_user_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM organization_memberships
        WHERE org_id = p_org_id AND user_id = p_user_id
    );
$$;

-- Recursive policy-уудыг функц ашигладаг болгож дахин үүсгэнэ.
DROP POLICY IF EXISTS organizations_member ON organizations;
CREATE POLICY organizations_member ON organizations
    USING (
        current_setting('app.user_role', true) = 'user'
        AND app_is_org_member(organizations.id, NULLIF(current_setting('app.user_id', true), '')::uuid)
    )
    WITH CHECK (
        current_setting('app.user_role', true) = 'user'
        AND app_is_org_member(organizations.id, NULLIF(current_setting('app.user_id', true), '')::uuid)
    );

DROP POLICY IF EXISTS org_memberships_member_select ON organization_memberships;
CREATE POLICY org_memberships_member_select ON organization_memberships
    FOR SELECT
    USING (
        current_setting('app.user_role', true) = 'user'
        AND app_is_org_member(organization_memberships.org_id, NULLIF(current_setting('app.user_id', true), '')::uuid)
    );
