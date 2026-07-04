-- Gerege Template Version 27.0
-- Restore the broad app-role grants that the up migration tightened, matching
-- the default privileges initdb hands out. Guarded on the app role existing so
-- it is a no-op on deployments without the initdb 'app_user' role.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        RETURN;
    END IF;

    GRANT INSERT, UPDATE, DELETE ON permissions TO app_user;
    GRANT UPDATE ON role_permissions TO app_user;
    GRANT INSERT, DELETE ON ai_prompts TO app_user;
    GRANT INSERT, UPDATE, DELETE ON ai_knowledge TO app_user;
END $$;
