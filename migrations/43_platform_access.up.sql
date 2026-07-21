-- Government Template Platform V3.0
-- Платформын хандалтын горим (access_mode: public|private). Singleton (id=1)
-- config хүснэгт — site_appearance-ийн адил RLS-гүй нийтийн тохиргоо, зөвхөн
-- superadmin UPDATE хийнэ. public: хэн ч Government SSO-оор нэвтэрч болно (одоогийн
-- зан төлөв); private: зөвхөн админаас урьдчилан бүртгэсэн (national_id/civil_id-
-- ээр тохирох) хэрэглэгч л нэвтэрнэ, бусад иргэн eID-ээр баталгаажсан ч 403 авна.

CREATE TABLE IF NOT EXISTS platform_settings (
    id          SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    access_mode TEXT NOT NULL DEFAULT 'public' CHECK (access_mode IN ('public', 'private')),
    updated_at  TIMESTAMPTZ
);

INSERT INTO platform_settings(id, access_mode) VALUES (1, 'public')
ON CONFLICT (id) DO NOTHING;

-- Defense-in-depth (site_appearance/17-тай ижил зарчим): RLS-гүй нийтийн config
-- тул DB-баталгаа нь app role-ийн table grant. App нь зөвхөн UPDATE (SetAccessMode)
-- хийдэг тул INSERT/DELETE-г хасна. 'app_user' role байхгүй бол no-op.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        RAISE NOTICE 'app_user role not found — skipping platform_settings privilege tightening';
        RETURN;
    END IF;
    REVOKE INSERT, DELETE ON platform_settings FROM app_user;
END $$;
