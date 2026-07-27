#!/usr/bin/env bash
#
# Synthetic SSO нэвтрэлтийн шалгалт — флот даяарх нэвтрэлт эвдэрсэн эсэхийг барина.
#
# ЯАГААД ХЭРЭГТЭЙ: 2026-07-27-нд Hydra-г өөрийн кодоор орлуулах явцад OAuth client
# бүртгэлүүд алдагдаж, sso.dgov.mn ба sso.gerege.mn ХОЁУЛАА тэг client-тэй үлдсэн.
# 7 платформын нэвтрэлт `invalid_client` өгч байсныг ХЭН Ч АНЗААРААГҮЙ — учир нь:
#   • CI зөвхөн build/test хардаг (ажиллаж буй системд хүрдэггүй),
#   • healthcheck зөвхөн /health хардаг (энэ нь SSO-г огт хөнддөггүй),
#   • нүүр хуудас 200 буцаасаар байсан.
# Энэ скрипт нь нэвтрэлтийн урсгалыг БОДИТООР гүйлгэж үзнэ.
#
# ЮУГ ШАЛГАХ ВЭ (хэрэглэгчийн эхний хоёр алхам):
#   1) GET  {платформ}/api/auth/sso/start   → 30x, Location = {sso}/oauth2/auth?client_id=…
#   2) GET  {authorize URL}                 → 302, Location-д `login_challenge=` байна
#
#   Эвдэрсэн үеийн гарын үсэг: 2-р алхам `400 {"error":"invalid_client"}` буцаана.
#
# Нэвтрэлтийг ДУУСГАХГҮЙ (PIN/eID шаардана) — тэр цэг хүртэл л явна. Гэхдээ яг
# энэ хоёр алхам нь client бүртгэл, issuer, redirect_uri, SSO-ийн амьд эсэхийг
# бүгдийг нь хамардаг.
#
# Хэрэглээ:
#   ./scripts/sso-login-probe.sh            # бүх платформ
#   ./scripts/sso-login-probe.sh gerege     # зөвхөн нэрэнд нь тохирохыг
#   ./scripts/sso-login-probe.sh --self-test  # шалгалт өөрөө эвдрэлийг барьж
#                                             # чадаж байгаа эсэхийг батална
# Гаралт: платформ бүрт нэг мөр; аль нэг унавал exit code 1.

set -uo pipefail

FILTER="${1:-}"
TIMEOUT=15

# Платформ бүр SSO-г ХЭРЭГЛЭГЧ (consumer) байдлаар ашигладаг. gerege.mn нь өөрөө
# provider тул энд байхгүй; wallet нь гар утасны eID урсгалтай тул мөн байхгүй.
PLATFORMS=(
  "template.dgov.mn"
  "ring.dgov.mn"
  "hurdan.dgov.mn"
  "developer.dgov.mn"
  "template.gerege.mn"
  "developer.gerege.mn"
  "geregeapp.mn"
  "geregepos.mn"
  "geregekiosk.mn"
)

# OIDC provider-ууд — discovery нь амьд эсэх (хямд, нэмэлт дохио).
PROVIDERS=(
  "sso.dgov.mn"
  "sso.gerege.mn"
  "gerege.mn"
)

pass=0
fail=0

red()   { printf '\033[31m%s\033[0m' "$1"; }
green() { printf '\033[32m%s\033[0m' "$1"; }

fail_line() {
  # $1=нэр  $2=шалтгаан
  printf '  %-24s %s  %s\n' "$1" "$(red FAIL)" "$2"
  fail=$((fail + 1))
}

check_platform() {
  local host="$1"
  local start="https://${host}/api/auth/sso/start"

  # ── 1-р алхам: платформ authorize руу чиглүүлж байна уу ──────────────────
  local out code loc
  out="$(curl -s -o /dev/null -w '%{http_code}|%{redirect_url}' --max-time "$TIMEOUT" "$start" 2>/dev/null)"
  code="${out%%|*}"
  loc="${out#*|}"

  if [ -z "$loc" ]; then
    fail_line "$host" "1-р алхам: $start → HTTP $code, redirect байхгүй"
    return
  fi

  local client_id sso_host
  client_id="$(printf '%s' "$loc" | grep -oE 'client_id=[^&]*' | cut -d= -f2)"
  sso_host="$(printf '%s' "$loc" | sed -E 's#https?://([^/]+).*#\1#')"

  if [ -z "$client_id" ]; then
    fail_line "$host" "1-р алхам: authorize URL-д client_id алга"
    return
  fi

  # ── 2-р алхам: SSO нь login_challenge гаргаж байна уу ────────────────────
  local a_code a_loc body
  a_code="$(curl -s -o /tmp/sso_probe_body -w '%{http_code}' --max-time "$TIMEOUT" "$loc" 2>/dev/null)"
  a_loc="$(curl -s -o /dev/null -w '%{redirect_url}' --max-time "$TIMEOUT" "$loc" 2>/dev/null)"

  case "$a_loc" in
    *login_challenge=*)
      printf '  %-24s %s  %s → %s\n' "$host" "$(green PASS)" "$client_id" "$sso_host"
      pass=$((pass + 1))
      return
      ;;
  esac

  body="$(head -c 200 /tmp/sso_probe_body 2>/dev/null | tr -d '\n')"
  case "$body" in
    *invalid_client*)
      fail_line "$host" "$(red 'invalid_client') — «$client_id» нь $sso_host дээр БҮРТГЭЛГҮЙ"
      ;;
    *)
      fail_line "$host" "2-р алхам: HTTP $a_code, login_challenge гараагүй ${body:+· $body}"
      ;;
  esac
}

check_provider() {
  local host="$1"
  local code issuer
  code="$(curl -s -o /tmp/sso_probe_oc -w '%{http_code}' --max-time "$TIMEOUT" \
          "https://${host}/.well-known/openid-configuration" 2>/dev/null)"
  if [ "$code" != "200" ]; then
    fail_line "$host" "OIDC discovery → HTTP $code"
    return
  fi
  issuer="$(python3 -c 'import json,sys; print(json.load(open("/tmp/sso_probe_oc")).get("issuer",""))' 2>/dev/null)"
  if [ "$issuer" != "https://${host}" ]; then
    fail_line "$host" "issuer зөрүүтэй: «$issuer»"
    return
  fi
  printf '  %-24s %s  issuer=%s\n' "$host" "$(green PASS)" "$issuer"
  pass=$((pass + 1))
}

# ── Өөрийгөө шалгах горим ────────────────────────────────────────────────
#
# «Ногоон» гэдэг нь ямар нэг зүйл ҮНЭХЭЭР шалгагдсаны дараа л утгатай. Энэ
# горим нь санаатайгаар байхгүй client-ээр authorize дуудаж, шалгалт үүнийг
# FAIL гэж барьж байгааг батална. Барихгүй бол скрипт өөрөө эвдэрсэн гэсэн үг —
# тэр тохиолдолд ногоон гэрэл худал болно.
if [ "$FILTER" = "--self-test" ]; then
  echo "── Өөрийгөө шалгах: эвдрэлийг барьж чадаж байна уу? ──────"
  probe_start="https://template.dgov.mn/api/auth/sso/start"
  real_loc="$(curl -s -o /dev/null -w '%{redirect_url}' --max-time "$TIMEOUT" "$probe_start" 2>/dev/null)"
  if [ -z "$real_loc" ]; then
    echo "  ✗ Өөрийгөө шалгах боломжгүй: authorize URL авч чадсангүй"
    exit 1
  fi
  bad_loc="$(printf '%s' "$real_loc" | sed 's/client_id=[^&]*/client_id=probe-nonexistent-client/')"
  bad_code="$(curl -s -o /tmp/sso_probe_body -w '%{http_code}' --max-time "$TIMEOUT" "$bad_loc" 2>/dev/null)"
  bad_body="$(head -c 200 /tmp/sso_probe_body 2>/dev/null | tr -d '\n')"
  rm -f /tmp/sso_probe_body
  case "$bad_body" in
    *invalid_client*)
      printf '  %s  байхгүй client → HTTP %s invalid_client (шалгалт үүнийг FAIL гэж барина)\n' \
             "$(green OK)" "$bad_code"
      echo
      echo "✓ Шалгалт эвдрэлийг барих чадвартай."
      exit 0
      ;;
  esac
  printf '  %s  байхгүй client → HTTP %s, invalid_client ГАРААГҮЙ: %s\n' \
         "$(red 'АНХААР')" "$bad_code" "$bad_body"
  echo
  echo "✗ Шалгалт эвдрэлийг барихгүй байж магадгүй — гарын үсгийг дахин тохируул."
  exit 1
fi

echo "── OIDC provider (discovery) ─────────────────────────────"
for h in "${PROVIDERS[@]}"; do
  [ -n "$FILTER" ] && case "$h" in *"$FILTER"*) ;; *) continue ;; esac
  check_provider "$h"
done

echo
echo "── Платформын нэвтрэлт (start → authorize → login_challenge) ──"
for h in "${PLATFORMS[@]}"; do
  [ -n "$FILTER" ] && case "$h" in *"$FILTER"*) ;; *) continue ;; esac
  check_platform "$h"
done

rm -f /tmp/sso_probe_body /tmp/sso_probe_oc

echo
if [ "$fail" -eq 0 ]; then
  printf '✓ Бүгд ажиллаж байна (%d шалгалт)\n' "$pass"
  exit 0
fi
printf '✗ %d шалгалт УНАВ (%d ногоон)\n' "$fail" "$pass"
exit 1
