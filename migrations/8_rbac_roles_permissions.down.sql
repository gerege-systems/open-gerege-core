-- Government Template Platform V3.0
-- Revert RBAC roles/permissions.

ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_role;

DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
