-- Модулийн идэвхийн төлөв (V4.0 Modular Platform, Phase 3 lifecycle).
-- Business модулийг restart-гүйгээр асааж/унтраах admin API-ийн ард байх
-- persistence: мөр бүр нэг модулийн төлөв. Мөргүй модуль = идэвхтэй (default).
--
-- ЗАРЧИМ: энэ хүснэгт нь БҮРТГЭЛ биш, зөвхөн ТӨЛӨВ. Модулиудын жагсаалт,
-- ангилал (core|business), хамаарлын граф нь кодын манифест (kernel/module
-- Builtin) — DB-ээс модуль "зохиох" боломжгүй, тиймээс stale мөр аюулгүй
-- (boot дээр таарахгүй ID зөвхөн warning). Core модулийн мөр үл тоогдоно —
-- core модуль унтардаггүй нь кодын инвариант.
--
-- site_appearance/platform_settings-тэй ижил: нийтийн config тул RLS-гүй;
-- бичих эрх нь route давхаргад RequireAdmin-аар хамгаалагдана.

CREATE TABLE IF NOT EXISTS platform_modules (
    id         TEXT PRIMARY KEY,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Defense-in-depth (43-тай ижил зарчим): app role нь SELECT/INSERT/UPDATE л
-- хийнэ (upsert) — DELETE хэрэггүй тул хасна. 'app_user' байхгүй бол no-op.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        RAISE NOTICE 'app_user role not found — skipping platform_modules privilege tightening';
        RETURN;
    END IF;
    REVOKE DELETE ON platform_modules FROM app_user;
END $$;
