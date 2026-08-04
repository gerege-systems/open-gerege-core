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
-- ЭНЭ ФАЙЛ ОДОО NO-OP.
--
-- Анх permissions / role_permissions / ai_prompts / ai_knowledge дээрх
-- REVOKE-уудыг агуулж байсан. Тэдгээр хүснэгтүүд модуль руу нүүсний дараа
-- энэ файл тэднээс ӨМНӨ ажилладаг болсон тул REVOKE-ууд нь эзэн модулиуд
-- руугаа нүүсэн:
--   permissions, role_permissions -> modules/rbac/migrations/54_rbac_config_grants
--   ai_prompts, ai_knowledge      -> modules/ai/migrations/53_ai_config_grants
--
-- Файлыг УСТГАХГҮЙ: production-д аль хэдийн хэрэгжсэн гэж бүртгэгдсэн тул
-- нэрийг нь хадгална (runner нь нэрээр түлхүүрлэдэг).
DO $$
BEGIN
    RETURN;
END $$;
