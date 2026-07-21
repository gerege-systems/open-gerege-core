-- Government Template Platform V3.0
-- RBAC: dynamic roles + a code-defined permission catalogue + role↔permission
-- assignments. The 'admin' role is not given explicit rows — the rbac usecase
-- resolves it to the FULL catalogue automatically. role ids are seeded to match
-- the existing users.role_id values (admin=1, user=2) and add manager=3.

CREATE TABLE IF NOT EXISTS roles (
    id          SERIAL PRIMARY KEY,
    key         TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_system   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS permissions (
    key      TEXT PRIMARY KEY,
    label    TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id        INT  NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_key TEXT NOT NULL REFERENCES permissions(key) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_key)
);

-- Permission catalogue (matches domain.AllPermissions).
INSERT INTO permissions(key, label, category) VALUES
    ('dashboard.view',  'Хяналтын самбар үзэх', 'general'),
    ('settings.manage', 'Тохиргоо удирдах',     'general'),
    ('users.manage',    'Хэрэглэгч удирдах',    'administration'),
    ('roles.manage',    'Эрх (role) удирдах',   'administration'),
    ('manager.view',    'Менежерийн хэсэг',     'management'),
    ('personal.view',   'Хувийн хэсэг',         'personal')
ON CONFLICT (key) DO NOTHING;

-- System roles with explicit ids matching users.role_id.
INSERT INTO roles(id, key, name, description, is_system) VALUES
    (1, 'admin',   'Админ',   'Бүх эрхтэй системийн админ', true),
    (2, 'user',    'Хэрэглэгч','Энгийн хэрэглэгч',          true),
    (3, 'manager', 'Менежер', 'Хэрэглэгч хянадаг менежер',  true)
ON CONFLICT (id) DO NOTHING;

-- Keep the SERIAL sequence ahead of the explicitly-seeded ids.
SELECT setval('roles_id_seq', (SELECT MAX(id) FROM roles));

-- Explicit grants. 'admin' is intentionally omitted — it auto-resolves to all.
INSERT INTO role_permissions(role_id, permission_key) VALUES
    -- user
    (2, 'dashboard.view'),
    (2, 'personal.view'),
    -- manager
    (3, 'dashboard.view'),
    (3, 'manager.view'),
    (3, 'users.manage')
ON CONFLICT DO NOTHING;

-- Integrity: users.role_id must reference a real role (ids already match).
ALTER TABLE users
    ADD CONSTRAINT fk_users_role FOREIGN KEY (role_id) REFERENCES roles(id);
