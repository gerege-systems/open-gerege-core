#!/bin/sh
# Gerege Template Version 27.0
#
# Postgres-ийн анхны init дээр НЭГ удаа (data volume хоосон үед) ажиллана. dan-ийг
# OIDC provider болгох Ory Hydra-д зориулж ТУСДАА `hydra` database үүсгэдэг — Hydra
# өөрийн schema/migration-ыг тэнд ажиллуулна (ssod-style провайдерын хүснэгтүүд нь
# үндсэн gerege_template DB-д migrate-ээр орно). Hydra нь POSTGRES_USER (superuser)-
# ээр холбогддог тул зөвхөн DB үүсгэхэд л хангалттай.
#
# ОДОО АЖИЛЛАЖ БУЙ (volume аль хэдийн байгаа) deployment дээр энэ script ажиллахгүй
# тул нэг удаа гараар:
#   docker compose exec db psql -U "$POSTGRES_USER" -c 'CREATE DATABASE hydra;'
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-'EOSQL'
	SELECT 'CREATE DATABASE hydra'
	WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'hydra')\gexec
EOSQL

echo "initdb: hydra database ready (Ory Hydra OIDC provider)"
