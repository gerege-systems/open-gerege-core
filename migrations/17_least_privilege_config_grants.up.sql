-- Government Template Platform V3.0
-- Defense-in-depth for the GLOBAL config tables (RBAC catalogue + AI prompts /
-- knowledge). These are not per-user tables, so they intentionally carry no
-- Row-Level Security — which means the ONLY DB-level backstop against a missed
-- handler authz check is the app role's table privileges. initdb grants the app
-- role broad SELECT/INSERT/UPDATE/DELETE on every table (it runs before these
-- tables exist, so it cannot be table-specific), so we narrow those grants here
-- to exactly what the repository layer actually uses. After this migration the
-- app connection cannot INSERT a new AI prompt key, rewrite the permission
-- catalogue, or tamper with the knowledge base even if an API authz check is
-- ever bypassed.
--
-- Guarded on the app role being named 'app_user' (the documented default —
-- APP_DB_USER). A deployment that uses a different role name, or an existing DB
-- provisioned without the initdb app role, is left untouched (no-op) and should
-- mirror these REVOKEs by hand for the same backstop. REVOKE of a not-held
-- privilege is a no-op, so re-running is safe.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        RAISE NOTICE 'app_user role not found — skipping config-table privilege tightening (custom APP_DB_USER? mirror these REVOKEs by hand)';
        RETURN;
    END IF;

    -- permissions: the catalogue is code/migration-defined; the app only reads
    -- it (rbac ListPermissions). No app write path exists.
    REVOKE INSERT, UPDATE, DELETE ON permissions FROM app_user;

    -- role_permissions: rbac replaces grants with DELETE + INSERT (no UPDATE);
    -- both columns form the PK so an UPDATE is meaningless anyway. roles itself
    -- keeps full CRUD — admins manage roles at runtime.
    REVOKE UPDATE ON role_permissions FROM app_user;

    -- ai_prompts: SetPrompt is UPDATE-only against the seeded keys, so the
    -- prompt surface must not grow or shrink through the app. Enforce it in the
    -- DB, not just the repository comment.
    REVOKE INSERT, DELETE ON ai_prompts FROM app_user;

    -- ai_knowledge: the app only runs the search_knowledge SELECT; content is
    -- seed/migration-managed, with no app write path.
    REVOKE INSERT, UPDATE, DELETE ON ai_knowledge FROM app_user;
END $$;
