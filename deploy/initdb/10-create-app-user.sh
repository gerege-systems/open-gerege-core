#!/bin/sh
# Gerege Template Version 27.0
#
# Postgres-ийн анхны init дээр НЭГ удаа (data volume хоосон үед) superuser
# POSTGRES_USER-ээр ажиллана. Хамгийн бага эрхтэй (least-privilege) application
# role үүсгэдэг — ингэснээр api нь NON-superuser-ээр холбогдож Row-Level Security
# бодит хэрэгжинэ (superuser болон BYPASSRLS role нь RLS-ийг алгасдаг).
#
# migrate контейнер нь POSTGRES_USER (superuser) хэвээр ашигладаг — CREATE
# EXTENSION "uuid-ossp", ALTER TABLE ... FORCE ROW LEVEL SECURITY, CREATE POLICY
# зэрэгт superuser/owner эрх шаардлагатай. Зөвхөн api л энэ хязгаарлагдмал
# role-оор холбогдоно.
set -e

: "${APP_DB_USER:?APP_DB_USER must be set for the least-privilege app role}"
: "${APP_DB_PASSWORD:?APP_DB_PASSWORD must be set for the least-privilege app role}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
	DO \$\$
	BEGIN
		IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${APP_DB_USER}') THEN
			CREATE ROLE ${APP_DB_USER} LOGIN PASSWORD '${APP_DB_PASSWORD}'
				NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
		END IF;
	END
	\$\$;

	GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO ${APP_DB_USER};
	GRANT USAGE ON SCHEMA public TO ${APP_DB_USER};

	-- migrate (POSTGRES_USER-ээр) дараа нь үүсгэх хүснэгт/sequence-ууд app
	-- role-д DML эрхийг автоматаар олгоно.
	ALTER DEFAULT PRIVILEGES FOR ROLE ${POSTGRES_USER} IN SCHEMA public
		GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO ${APP_DB_USER};
	ALTER DEFAULT PRIVILEGES FOR ROLE ${POSTGRES_USER} IN SCHEMA public
		GRANT USAGE, SELECT ON SEQUENCES TO ${APP_DB_USER};

	-- init үед аль хэдийн байгаа аливаа объект (цэвэр volume дээр байхгүй ч,
	-- байгаа schema-руу дахин ажиллуулахад зөв байлгана).
	GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO ${APP_DB_USER};
	GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO ${APP_DB_USER};

	-- Тэмдэглэл: эдгээр нь өргөн (бүх хүснэгтэд DML) default. RLS-гүй глобал
	-- config хүснэгтүүд (permissions / role_permissions / ai_prompts /
	-- ai_knowledge) дээрх эрхийг migration 17 нь repo-ийн бодит хэрэглээнд
	-- нийцүүлж (defense-in-depth) багасгана. Тэр migration нь role нэрийг
	-- 'app_user' гэж үздэг тул APP_DB_USER-г өөр нэрээр тохируулбал тэнд
	-- заасан REVOKE-уудыг гараар давтана уу.
EOSQL

echo "initdb: least-privilege role '${APP_DB_USER}' (NOSUPERUSER NOBYPASSRLS) ready"
