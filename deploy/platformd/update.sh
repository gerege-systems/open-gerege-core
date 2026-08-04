#!/usr/bin/env bash
# Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.
#
# platformd-ийн apply script — $GEREGE_TARGET_VERSION таг руу шилжиж стекийг
# дахин босгоно. Репо root-оос (docker-compose.yml байгаа газраас) ажиллана.
# Rollback-ийн тулд өмнөх байрлалыг .platformd-prev файлд хадгална.
set -euo pipefail

: "${GEREGE_TARGET_VERSION:?GEREGE_TARGET_VERSION env шаардлагатай}"

echo "==> update: $(git describe --tags --always) -> ${GEREGE_TARGET_VERSION}"

# Одоогийн байрлалыг rollback-д хадгална.
git rev-parse HEAD > .platformd-prev

git fetch --tags origin
git checkout --quiet "${GEREGE_TARGET_VERSION}"

# api + web-ийг дахин build хийж босгоно; db/redis-т хүрэхгүй.
# migrate service нь `up`-д нэг удаа гүйж шинэ migration-уудыг апплай хийнэ.
docker compose up -d --build migrate api web

echo "==> update: containers up, health шалгалтыг platformd хийнэ"
