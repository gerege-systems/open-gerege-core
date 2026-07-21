-- Government Template Platform V3.0
-- Reverse 23_superadmin_role: restore the original scheme (admin=1, user=2,
-- manager=3) and drop the superadmin role. Any super admins are demoted to admin
-- first so the users.role_id → roles(id) foreign key does not block the change.

-- 1. Drop the FKs so the primary keys can be renumbered back.
ALTER TABLE users            DROP CONSTRAINT fk_users_role;
ALTER TABLE role_permissions DROP CONSTRAINT role_permissions_role_id_fkey;

-- 2. Demote super admins (role_id=1) to admin and remove the superadmin role.
UPDATE users SET role_id = 2, updated_at = now() WHERE role_id = 1;
DELETE FROM roles WHERE id = 1 AND key = 'superadmin';

-- 3. Shift remaining roles to a temporary range (+100).
UPDATE roles            SET id      = id      + 100 WHERE id      BETWEEN 1 AND 99;
UPDATE users            SET role_id = role_id + 100 WHERE role_id BETWEEN 1 AND 99;
UPDATE role_permissions SET role_id = role_id + 100 WHERE role_id BETWEEN 1 AND 99;

-- 4. Reverse remap: admin 102→1, user 104→2, manager 103→3.
UPDATE roles            SET id      = CASE id      WHEN 102 THEN 1 WHEN 104 THEN 2 WHEN 103 THEN 3 ELSE id      END WHERE id      BETWEEN 100 AND 199;
UPDATE users            SET role_id = CASE role_id WHEN 102 THEN 1 WHEN 104 THEN 2 WHEN 103 THEN 3 ELSE role_id END WHERE role_id BETWEEN 100 AND 199;
UPDATE role_permissions SET role_id = CASE role_id WHEN 102 THEN 1 WHEN 104 THEN 2 WHEN 103 THEN 3 ELSE role_id END WHERE role_id BETWEEN 100 AND 199;

-- 5. Fix the sequence and re-add the foreign keys.
SELECT setval('roles_id_seq', (SELECT MAX(id) FROM roles));
ALTER TABLE users            ADD CONSTRAINT fk_users_role FOREIGN KEY (role_id) REFERENCES roles(id);
ALTER TABLE role_permissions ADD CONSTRAINT role_permissions_role_id_fkey FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE;
