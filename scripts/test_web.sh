#!/bin/sh
set -u

WEB_URL=${WEB_URL:-https://localhost:8443}
TMP_DIR=$(mktemp -d)
BODY="$TMP_DIR/body"
HEADERS="$TMP_DIR/headers"
COOKIE="$TMP_DIR/cookies"
ADMIN_COOKIE="$TMP_DIR/admin-cookies"
PASSED=0
NORMAL=0
FAILED=0
GREEN=$(printf '\033[32m')
YELLOW=$(printf '\033[33m')
RED=$(printf '\033[31m')
RESET=$(printf '\033[0m')
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

for tool in curl jq grep; do
  command -v "$tool" >/dev/null 2>&1 || { echo "$tool is required." >&2; exit 127; }
done

ok() { PASSED=$((PASSED + 1)); printf '%s: %s[OK]%s\n' "$1" "$GREEN" "$RESET"; }
normal() { NORMAL=$((NORMAL + 1)); printf '%s: %s[NORMAL]%s\n' "$1" "$YELLOW" "$RESET"; }
fail() {
  FAILED=$((FAILED + 1))
  printf '%s: %s[ERR]%s\n' "$1" "$RED" "$RESET"
  printf '%s caught error %s:\n> Expected: %s\n> Got: %s\n' "$1" "$2" "$3" "$4"
}

html_get() {
  PATHNAME=$1; JAR=${2:-}
  set -- -k -sS -o "$BODY" -D "$HEADERS" -w '%{http_code}'
  [ -z "$JAR" ] || set -- "$@" -b "$JAR" -c "$JAR"
  STATUS=$(curl "$@" "$WEB_URL$PATHNAME")
}

form_post() {
  PATHNAME=$1; JAR=${2:-}; ORIGIN=${3:-}; shift 3 || true
  set -- -k -sS -o "$BODY" -D "$HEADERS" -w '%{http_code}' -X POST "$@"
  [ -z "$JAR" ] || set -- "$@" -b "$JAR" -c "$JAR"
  [ -z "$ORIGIN" ] || set -- "$@" -H "Origin: $ORIGIN"
  STATUS=$(curl "$@" "$WEB_URL$PATHNAME")
}

api_request() {
  METHOD=$1; PATHNAME=$2; JAR=${3:-}; ORIGIN=${4:-}; DATA=${5:-}
  set -- -k -sS -o "$BODY" -D "$HEADERS" -w '%{http_code}' -X "$METHOD" -H 'Accept: application/json'
  [ -z "$JAR" ] || set -- "$@" -b "$JAR" -c "$JAR"
  [ -z "$ORIGIN" ] || set -- "$@" -H "Origin: $ORIGIN"
  [ -z "$DATA" ] || set -- "$@" -H 'Content-Type: application/json' --data "$DATA"
  STATUS=$(curl "$@" "$WEB_URL$PATHNAME")
}

json() { jq -e "$@" "$BODY" >/dev/null 2>&1; }
header() { grep -Eiq "$1" "$HEADERS"; }

ssr_shell() {
  N=SSRWebShell
  html_get '/'
  if [ "$STATUS" != 200 ] || ! grep -Fq '/assets/site.css' "$BODY" || ! grep -Fq '/assets/site.js' "$BODY" || ! grep -Fq 'Featured' "$BODY"; then
    fail "$N" HtmlShell '200 server-rendered shell with embedded assets' "$STATUS $(head -c 300 "$BODY")"; return
  fi
  if grep -Fq '/app.js' "$BODY" || grep -Fq 'data-route=' "$BODY"; then
    fail "$N" NoSPA 'no legacy SPA entrypoint or client router' "$(head -c 300 "$BODY")"; return
  fi
  if ! header '^Content-Security-Policy:' || ! header '^X-Frame-Options:[[:space:]]*DENY'; then
    fail "$N" SecurityHeaders 'CSP and DENY frame policy' "$(cat "$HEADERS")"; return
  fi
  html_get '/assets/site.js'
  if [ "$STATUS" != 200 ] || grep -Eq 'fetch[[:space:]]*\(|Promise|=>|(^|[^[:alnum:]_])(const|let|async)[[:space:]]' "$BODY"; then
    fail "$N" Safari5JS 'ES5-only progressive enhancement' "$STATUS $(head -c 300 "$BODY")"; return
  fi
  ok "$N"
}

auth_modes() {
  N=SSRAuthModes
  html_get '/account?mode=login'
  [ "$STATUS" = 200 ] && grep -Fq 'Нет аккаунта?' "$BODY" && grep -Fq 'Забыли пароль?' "$BODY" || { fail "$N" LoginMode 'login links present' "$STATUS $(head -c 300 "$BODY")"; return; }
  html_get '/account?mode=register'
  [ "$STATUS" = 200 ] && grep -Fq 'Создать аккаунт' "$BODY" && grep -Fq 'Уже есть аккаунт?' "$BODY" || { fail "$N" RegisterMode 'registration form present' "$STATUS $(head -c 300 "$BODY")"; return; }
  html_get '/account?mode=recovery'
  [ "$STATUS" = 200 ] && grep -Fq 'Восстановление пароля' "$BODY" || { fail "$N" RecoveryMode 'recovery form present' "$STATUS $(head -c 300 "$BODY")"; return; }
  ok "$N"
}

ssr_account_lifecycle() {
  N=SSRAccountLifecycle
  EMAIL="ssr-web-$(date +%s)-$$@example.invalid"
  PASSWORD='SSR-Web-Password-2026!'

  form_post '/account/register' "$COOKIE" "$WEB_URL" \
    --data-urlencode "email=$EMAIL" \
    --data-urlencode 'nickname=SSR Browser User' \
    --data-urlencode "password=$PASSWORD" \
    --data-urlencode 'remember_me=1'
  if [ "$STATUS" != 303 ] || ! header '^Set-Cookie:[[:space:]]*legacystore_session=' || ! header 'HttpOnly' || ! header 'Secure'; then
    fail "$N" Register '303 and secure session cookie' "$STATUS $(cat "$HEADERS")"; return
  fi

  html_get '/account/profile' "$COOKIE"
  if [ "$STATUS" != 200 ] || ! grep -Fq "$EMAIL" "$BODY" || ! grep -Fq 'Активные сессии' "$BODY"; then
    fail "$N" Profile 'authenticated SSR profile' "$STATUS $(head -c 300 "$BODY")"; return
  fi

  form_post '/account/profile' "$COOKIE" "$WEB_URL" \
    --data-urlencode 'nickname=SSR Updated User' \
    --data-urlencode 'avatar_url='
  [ "$STATUS" = 303 ] || { fail "$N" UpdateProfile 303 "$STATUS $(head -c 300 "$BODY")"; return; }
  html_get '/account/profile' "$COOKIE"
  grep -Fq 'SSR Updated User' "$BODY" || { fail "$N" UpdatedProfile 'updated nickname rendered by server' "$(head -c 300 "$BODY")"; return; }

  form_post '/account/profile' "$COOKIE" 'https://evil.example' \
    --data-urlencode 'nickname=Rejected'
  if [ "$STATUS" != 403 ]; then
    fail "$N" CSRF '403 on cross-origin cookie form post' "$STATUS $(head -c 300 "$BODY")"; return
  fi
  ok "$N"
}

catalog_pages() {
  N=SSRCatalogPages
  html_get '/search?q=Pixelmator'
  if [ "$STATUS" != 200 ] || ! grep -Fq 'Pixelmator' "$BODY"; then
    fail "$N" Search 'SSR search results' "$STATUS $(head -c 300 "$BODY")"; return
  fi
  html_get '/app/pixelmator'
  if [ "$STATUS" != 200 ] || ! grep -Fq 'Описание' "$BODY" || ! grep -Fq 'Версии' "$BODY"; then
    fail "$N" AppDetail 'SSR application detail with versions' "$STATUS $(head -c 300 "$BODY")"; return
  fi
  ok "$N"
}

admin_pages() {
  N=SSRAdminPages
  AE=${API_TEST_ADMIN_EMAIL:-}; AP=${API_TEST_ADMIN_PASSWORD:-}
  if [ -z "$AE" ] || [ -z "$AP" ]; then normal "$N"; return; fi

  form_post '/account/login' "$ADMIN_COOKIE" "$WEB_URL" \
    --data-urlencode "email=$AE" \
    --data-urlencode "password=$AP" \
    --data-urlencode 'remember_me=1'
  [ "$STATUS" = 303 ] || { fail "$N" AdminLogin 303 "$STATUS $(head -c 300 "$BODY")"; return; }

  html_get '/admin' "$ADMIN_COOKIE"
  [ "$STATUS" = 200 ] && grep -Fq 'Admin Dashboard' "$BODY" || { fail "$N" Dashboard 'admin SSR dashboard' "$STATUS $(head -c 300 "$BODY")"; return; }
  html_get '/admin/moderation' "$ADMIN_COOKIE"
  [ "$STATUS" = 200 ] && grep -Fq 'Модерация' "$BODY" || { fail "$N" Moderation 'moderation SSR page' "$STATUS $(head -c 300 "$BODY")"; return; }
  html_get '/admin/system?tab=users' "$ADMIN_COOKIE"
  [ "$STATUS" = 200 ] && grep -Fq 'Создать аккаунт' "$BODY" || { fail "$N" SystemUsers 'user management SSR page' "$STATUS $(head -c 300 "$BODY")"; return; }
  html_get '/admin/system?tab=apps' "$ADMIN_COOKIE"
  [ "$STATUS" = 200 ] && grep -Fq 'Создать приложение' "$BODY" || { fail "$N" SystemApps 'application management SSR page' "$STATUS $(head -c 300 "$BODY")"; return; }
  html_get '/admin/system?tab=uploads' "$ADMIN_COOKIE"
  [ "$STATUS" = 200 ] && grep -Fq 'Создать запись загрузки' "$BODY" || { fail "$N" SystemUploads 'upload management SSR page' "$STATUS $(head -c 300 "$BODY")"; return; }
  html_get '/admin/system?tab=reviews' "$ADMIN_COOKIE"
  [ "$STATUS" = 200 ] && grep -Fq 'Создать комментарий' "$BODY" || { fail "$N" SystemReviews 'review management SSR page' "$STATUS $(head -c 300 "$BODY")"; return; }
  ok "$N"
}

proxy_bootstrap() {
  N=WebProxyBootstrap
  api_request GET '/api/v1/bootstrap'
  if [ "$STATUS" != 200 ] || ! json '.server_status == "ok" and .api_version == "v1"'; then
    fail "$N" BackendProxy '200 server_status=ok' "$STATUS $(cat "$BODY")"; return
  fi
  header '^Strict-Transport-Security:' || { fail "$N" HSTS 'HSTS on HTTPS response' "$(cat "$HEADERS")"; return; }
  ok "$N"
}

plain_http_rejected() {
  N=WebPlainHTTPAuthRejected
  case "$WEB_URL" in
    https://localhost:8443|https://127.0.0.1:8443)
      HTTP_URL=$(printf '%s' "$WEB_URL" | sed 's#^https://#http://#; s#:8443#:8081#') ;;
    *) normal "$N"; return ;;
  esac
  STATUS=$(curl -sS -o "$BODY" -w '%{http_code}' -X POST --data-urlencode 'email=plain@example.invalid' --data-urlencode 'password=unused' "$HTTP_URL/account/login")
  if [ "$STATUS" != 403 ]; then
    fail "$N" HTTPSPolicy '403 for SSR credential post over plain HTTP' "$STATUS $(head -c 300 "$BODY")"; return
  fi
  ok "$N"
}

ssr_shell
auth_modes
ssr_account_lifecycle
catalog_pages
admin_pages
proxy_bootstrap
plain_http_rejected

if [ "$FAILED" -eq 0 ]; then echo 'All SSR web integration tests passed.'; else echo 'SSR web integration tests finished with errors.'; fi
echo 'Summary:'
printf -- '- Passed: %s\n- Normal: %s\n- Failed: %s\n' "$PASSED" "$NORMAL" "$FAILED"
[ "$FAILED" -eq 0 ] || exit 1
