-- Government Template Platform V3.0
-- Revert the organizations + membership feature (RLS policies, tables, indexes).

DROP POLICY IF EXISTS org_memberships_member_select ON organization_memberships;
DROP POLICY IF EXISTS org_memberships_admin ON organization_memberships;
DROP POLICY IF EXISTS org_memberships_service ON organization_memberships;
DROP POLICY IF EXISTS organizations_member ON organizations;
DROP POLICY IF EXISTS organizations_admin ON organizations;
DROP POLICY IF EXISTS organizations_service ON organizations;

ALTER TABLE IF EXISTS organization_memberships NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS organization_memberships DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS organizations NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS organizations DISABLE ROW LEVEL SECURITY;

DROP TABLE IF EXISTS organization_memberships;

DROP INDEX IF EXISTS idx_organizations_created_by;
DROP INDEX IF EXISTS idx_organizations_reg_no_lower;
DROP TABLE IF EXISTS organizations;
