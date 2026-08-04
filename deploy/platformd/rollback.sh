#!/usr/bin/env bash
# Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.
#
# platformd-ийн rollback script — update.sh-ийн хадгалсан өмнөх commit руу
# буцаж стекийг сэргээнэ. Migration-ууд expand-contract зарчмаар бичигдсэн
# байх ёстой (доош буулгах шаардлагагүй) — MODULAR_REFACTOR_PLAN §4 Phase 5.
set -euo pipefail

if [[ ! -f .platformd-prev ]]; then
  echo "!! rollback: .platformd-prev алга — гар оролцоо хэрэгтэй" >&2
  exit 1
fi

PREV="$(cat .platformd-prev)"
echo "==> rollback: ${GEREGE_TARGET_VERSION:-?} -> ${PREV}"

git checkout --quiet "${PREV}"
docker compose up -d --build api web

echo "==> rollback дууслаа"
