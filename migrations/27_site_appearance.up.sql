-- Government Template Platform V3.0
-- Сайтын нийтийн харагдацын default (accent · font · style · theme). Админ
-- 'settings.manage' эрхээр өөрчилдөг, бүх зочин (landing + нэвтэрсэн) энэ
-- default-оор эхэлнэ; хэрэглэгч өөрийн тохиргоогоор дарж болно (client-side).
--
-- Ганц мөрт (singleton) хүснэгт — id = 1 гэсэн CHECK-ээр цоожилно. App нь
-- зөвхөн UPDATE хийдэг (SetAppearance) тул мөр нэмэгдэх/устахгүй; INSERT/DELETE-г
-- 17-р шилжилтийн адил доор REVOKE-оор хаана. RLS хэрэггүй — per-user биш,
-- нийтийн config.
CREATE TABLE IF NOT EXISTS site_appearance (
    id         SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    -- accent нь preset нэр ('cobalt'|'teal'|'violet'|'emerald'|'amber') ЭСВЭЛ
    -- '#rrggbb' custom hex. Баталгаажуулалт нь usecase/handler давхаргад.
    accent     TEXT NOT NULL DEFAULT 'cobalt',
    font       TEXT NOT NULL DEFAULT 'inter',
    style      TEXT NOT NULL DEFAULT 'comfortable',
    theme      TEXT NOT NULL DEFAULT 'light',
    updated_at TIMESTAMPTZ
);

INSERT INTO site_appearance(id, accent, font, style, theme)
VALUES (1, 'cobalt', 'inter', 'comfortable', 'light')
ON CONFLICT (id) DO NOTHING;

-- Defense-in-depth: RLS-гүй нийтийн config тул цорын ганц DB-баталгаа нь app
-- role-ийн table grant. App нь зөвхөн UPDATE хийдэг тул INSERT/DELETE-г хасна
-- (17_least_privilege_config_grants-тэй ижил зарчим). 'app_user' role байхгүй
-- бол no-op.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        RAISE NOTICE 'app_user role not found — skipping site_appearance privilege tightening';
        RETURN;
    END IF;
    REVOKE INSERT, DELETE ON site_appearance FROM app_user;
END $$;
