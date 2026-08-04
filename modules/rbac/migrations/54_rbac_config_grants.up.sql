-- rbac модулийн каталогийн хүснэгтүүдийн least-privilege grant-ууд.
--
-- ЯАГААД ЭНД: эдгээр REVOKE нь өмнө нь глобал
-- 17_least_privilege_config_grants-д байсан. permissions / role_permissions
-- нь rbac модульд нүүсний дараа глобал 17 нь тэдгээр хүснэгт үүсэхээс ӨМНӨ
-- ажиллах болсон. Модулийн grant нь модулийнхөө хүснэгттэй хамт нүүнэ.
--
-- Ажиллаж буй DB-д 17 нь эдгээр REVOKE-той хамт хэрэгжсэн байгаа; энэ файл
-- тэнд adopt хийгдэх тул давхардахгүй. REVOKE нь эзэмшээгүй эрхэд no-op.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        RAISE NOTICE 'app_user role not found — skipping rbac config-table privilege tightening';
        RETURN;
    END IF;

    -- permissions: каталог нь migration-аар удирдагдана; апп зөвхөн уншина.
    REVOKE INSERT, UPDATE, DELETE ON permissions FROM app_user;

    -- role_permissions: rbac нь DELETE + INSERT-ээр солидог (UPDATE хэрэггүй —
    -- хоёр багана хоёулаа PK). roles өөрөө бүрэн CRUD хэвээр.
    REVOKE UPDATE ON role_permissions FROM app_user;
END $$;
