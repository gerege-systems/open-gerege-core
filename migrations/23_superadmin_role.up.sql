-- Government Template Platform V3.0
-- Introduce a 'superadmin' tier ABOVE 'admin' and renumber the system roles so
-- ids reflect the privilege ladder (1 = highest):
--     superadmin=1, admin=2, manager=3, user=4
-- Migration 8 originally seeded admin=1, user=2, manager=3, so this migration
-- REMAPS existing rows (roles + users.role_id + role_permissions.role_id) and
-- seeds the new superadmin role. It runs cleanly on both a fresh DB (8 seeds the
-- old scheme, then this remaps) and an already-migrated DB.
--
-- BREAKING CHANGE: role_id meanings shift, so JWTs issued before this migration
-- are reinterpreted (old admin=1 → superadmin, old user=2 → admin). On an
-- existing deployment ROTATE JWT_SECRET (or force all users to re-login) when
-- applying this, otherwise stale tokens gain the wrong privilege.
--
-- Assumes only the seeded system roles (1,2,3) exist; any admin-created custom
-- roles keep referential integrity but land in a temporary 10x id range.

-- 1. Drop the FKs that reference roles(id) so the primary keys can be renumbered.
ALTER TABLE users            DROP CONSTRAINT fk_users_role;
ALTER TABLE role_permissions DROP CONSTRAINT role_permissions_role_id_fkey;

-- 2. Shift every existing role id into a temporary, non-colliding range (+100).
--    The same shift is applied to all referencing columns so integrity holds.
UPDATE roles            SET id      = id      + 100 WHERE id      BETWEEN 1 AND 99;
UPDATE users            SET role_id = role_id + 100 WHERE role_id BETWEEN 1 AND 99;
UPDATE role_permissions SET role_id = role_id + 100 WHERE role_id BETWEEN 1 AND 99;

-- 3. Remap to the new scheme: admin 101→2, user 102→4, manager 103→3 (unchanged).
UPDATE roles            SET id      = CASE id      WHEN 101 THEN 2 WHEN 102 THEN 4 WHEN 103 THEN 3 ELSE id      END WHERE id      BETWEEN 100 AND 199;
UPDATE users            SET role_id = CASE role_id WHEN 101 THEN 2 WHEN 102 THEN 4 WHEN 103 THEN 3 ELSE role_id END WHERE role_id BETWEEN 100 AND 199;
UPDATE role_permissions SET role_id = CASE role_id WHEN 101 THEN 2 WHEN 102 THEN 4 WHEN 103 THEN 3 ELSE role_id END WHERE role_id BETWEEN 100 AND 199;

-- 4. Seed the new top-tier superadmin role at id=1. Like 'admin', it is given no
--    explicit role_permissions rows — the rbac usecase resolves it to the FULL
--    catalogue automatically (super admin is admin-level and above).
INSERT INTO roles(id, key, name, description, is_system) VALUES
    (1, 'superadmin', 'Супер админ', 'Админуудыг удирдах дээд эрх', true)
ON CONFLICT (id) DO NOTHING;

-- 5. Keep the SERIAL sequence ahead of the explicitly-seeded ids.
SELECT setval('roles_id_seq', (SELECT MAX(id) FROM roles));

-- 6. Re-add the foreign keys.
ALTER TABLE users            ADD CONSTRAINT fk_users_role FOREIGN KEY (role_id) REFERENCES roles(id);
ALTER TABLE role_permissions ADD CONSTRAINT role_permissions_role_id_fkey FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE;
