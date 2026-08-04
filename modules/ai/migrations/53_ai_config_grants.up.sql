-- ai модулийн config хүснэгтүүдийн least-privilege grant-ууд.
--
-- ЯАГААД ЭНД: эдгээр REVOKE нь өмнө нь глобал
-- 17_least_privilege_config_grants-д байсан. ai_prompts / ai_knowledge нь
-- ai модульд нүүсний дараа глобал 17 нь тэдгээр хүснэгт үүсэхээс ӨМНӨ
-- ажиллах болсон тул "relation does not exist" алдаа өгдөг байв. Модулийн
-- өөрийн grant нь модулийнхөө хүснэгттэй хамт нүүх ёстой.
--
-- Аль хэдийн ажиллаж буй DB-д 17 нь эдгээр REVOKE-той хамт хэрэгжсэн
-- байгаа; энэ файл тэнд adopt хийгдэх (ажиллахгүй) тул давхардал үүсэхгүй.
-- REVOKE нь эзэмшээгүй эрхэд no-op тул дахин ажиллуулахад ч аюулгүй.
--
-- 17-гийнхтэй ижил хамгаалалт: app role нь 'app_user' нэртэй үед л үйлчилнэ.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        RAISE NOTICE 'app_user role not found — skipping ai config-table privilege tightening';
        RETURN;
    END IF;

    -- ai_prompts: SetPrompt is UPDATE-only against the seeded keys, so the
    -- prompt surface must not grow or shrink through the app.
    REVOKE INSERT, DELETE ON ai_prompts FROM app_user;

    -- ai_knowledge: the app only runs the search_knowledge SELECT; content is
    -- seed/migration-managed, with no app write path.
    REVOKE INSERT, UPDATE, DELETE ON ai_knowledge FROM app_user;
END $$;
