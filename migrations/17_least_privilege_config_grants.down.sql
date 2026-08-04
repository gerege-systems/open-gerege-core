-- Restore the broad app-role grants that the up migration tightened, matching
-- the default privileges initdb hands out. Guarded on the app role existing so
-- it is a no-op on deployments without the initdb 'app_user' role.
-- ЭНЭ ФАЙЛ ОДОО NO-OP — up-ийн тайлбарыг үз.
DO $$
BEGIN
    RETURN;
END $$;
