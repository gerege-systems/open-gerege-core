-- Government Template Platform V3.0
-- Revert Row-Level Security on the users table.

DROP POLICY IF EXISTS users_self ON users;
DROP POLICY IF EXISTS users_admin ON users;
DROP POLICY IF EXISTS users_service ON users;

ALTER TABLE users NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
