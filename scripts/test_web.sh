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

request() {
  METHOD=$1; PATHNAME=$2; JAR=${3:-}; ORIGIN=${4:-}; DATA=${5:-}
  set -- -k -sS -o "$BODY" -D "$HEADERS" -w '%{http_code}' -X "$METHOD" -H 'Accept: application/json'
  [ -z "$JAR" ] || set -- "$@" -b "$JAR" -c "$JAR"
  [ -z "$ORIGIN" ] || set -- "$@" -H "Origin: $ORIGIN"
  [ -z "$DATA" ] || set -- "$@" -H 'Content-Type: application/json' --data "$DATA"
  STATUS=$(curl "$@" "$WEB_URL$PATHNAME")
}

static_get() {
  STATUS=$(curl -k -sS -o "$BODY" -D "$HEADERS" -w '%{http_code}' "$WEB_URL$1")
}

json() { jq -e "$@" "$BODY" >/dev/null 2>&1; }
value() { jq -r "$1" "$BODY" 2>/dev/null; }
header() { grep -Eiq "$1" "$HEADERS"; }

web_shell() {
  N=WebShell
  static_get '/'
  if [ "$STATUS" != 200 ] || ! grep -q '/app.js' "$BODY" || ! grep -q '/ui-fixes.css' "$BODY" || ! grep -q '<svg' "$BODY"; then
    fail "$N" HtmlShell '200 with app.js, ui-fixes.css and SVG icons' "$STATUS $(head -c 300 "$BODY")"; return
  fi
  if grep -Fq 'data-route="downloads"' "$BODY" || grep -Fq 'data-route="updates"' "$BODY"; then
    fail "$N" WebOnlyNavigation 'no Downloads or Updates toolbar tabs in web UI' 'legacy-client-only toolbar route found'; return
  fi
  if ! header '^Content-Security-Policy:' || ! header '^X-Frame-Options:[[:space:]]*DENY'; then
    fail "$N" SecurityHeaders 'CSP and DENY frame policy' "$(cat "$HEADERS")"; return
  fi
  ok "$N"
}

frontend_bundle() {
  N=FrontendBundle
  static_get '/app.js'
  [ "$STATUS" = 200 ] || { fail "$N" HttpStatus 200 "$STATUS"; return; }
  for marker in '/me/password' '/me/email' '/auth/recovery/request' '/auth/2fa/recovery-codes/regenerate' '/admin/versions/' '/admin/uploads/inspect' '/home?' 'homeCarousel' 'Popular' 'Top Downloads' 'New Releases' 'artifactDropZone' 'uploadDropOverlay' 'showToast' 'recovery_code' 'app-icon-image' 'os_series=1' 'artifactPatchRequirementNote' 'Supported systems:'; do
    grep -Fq "$marker" "$BODY" || { fail "$N" MissingIntegration "$marker" 'not found'; return; }
  done
  if grep -Fq 'New and Noteworthy' "$BODY" || grep -Fq 'panel("Graphics & Design"' "$BODY"; then
    fail "$N" LegacyHomeSections 'Popular, Top Downloads and New Releases only' 'legacy home section found'; return
  fi
  if grep -Fq 'document.cookie' "$BODY"; then
    fail "$N" SessionSecurity 'server-owned HttpOnly session cookie' 'document.cookie found'; return
  fi
  if grep -Fq 'os: "10.9.5"' "$BODY" || grep -Fq '"10.9.5", "10.8"' "$BODY" || grep -Fq '"10.6.8", "10.5.8"' "$BODY" || grep -Fq '"10.5.8", "10.4.11"' "$BODY"; then
    fail "$N" CatalogOSSeries 'major.minor-only web catalog filters' 'patch-level value found in catalog filter definitions'; return
  fi
  if grep -Fq 'function renderDownloads' "$BODY" || grep -Fq 'function renderUpdates' "$BODY"; then
    fail "$N" WebOnlyNavigation 'browser-managed downloads and no update manager' 'legacy-client-only web view found'; return
  fi
  static_get '/ui-fixes.css'
  if [ "$STATUS" != 200 ] || ! grep -Fq '.app-icon-image' "$BODY" || ! grep -Fq '.tab-icon' "$BODY" || ! grep -Fq '.upload-bento' "$BODY" || ! grep -Fq '.bubble-toast' "$BODY" || ! grep -Fq '.field-totp' "$BODY" || ! grep -Fq '.home-carousel' "$BODY" || ! grep -Fq '.finder-mark' "$BODY"; then
    fail "$N" IconStyles 'stable icons, bento forms and carousel styles' "$STATUS $(head -c 300 "$BODY")"; return
  fi
  ok "$N"
}

proxy_bootstrap() {
  N=WebProxyBootstrap
  request GET '/api/v1/bootstrap'
  if [ "$STATUS" != 200 ] || ! json '.server_status == "ok" and .api_version == "v1"'; then
    fail "$N" BackendProxy '200 server_status=ok' "$STATUS $(cat "$BODY")"; return
  fi
  header '^Strict-Transport-Security:' || { fail "$N" HSTS 'HSTS on HTTPS response' "$(cat "$HEADERS")"; return; }
  ok "$N"
}

account_lifecycle() {
  N=WebAccountLifecycle
  EMAIL="web-$(date +%s)-$$@example.invalid"
  P1='Web-Test-Password-2026!'
  P2='Web-Test-Password-Changed-2026!'

  request POST '/api/v1/auth/register' "$COOKIE" "$WEB_URL" "{\"email\":\"$EMAIL\",\"nickname\":\"Web Integration\",\"password\":\"$P1\",\"remember_me\":true}"
  if [ "$STATUS" != 200 ] || ! json --arg email "$EMAIL" '.user.email == $email'; then
    fail "$N" Register '200 matching user' "$STATUS $(cat "$BODY")"; return
  fi
  if ! header '^Set-Cookie:[[:space:]]*legacystore_session=' || ! header 'HttpOnly' || ! header 'Secure' || ! header 'SameSite=Lax'; then
    fail "$N" SessionCookie 'Secure HttpOnly SameSite=Lax' "$(cat "$HEADERS")"; return
  fi

  request GET '/api/v1/me' "$COOKIE"
  if [ "$STATUS" != 200 ] || ! json --arg email "$EMAIL" '.user.email == $email and .session.auth_kind == "web"'; then
    fail "$N" CookieSession 'authenticated cookie session' "$STATUS $(cat "$BODY")"; return
  fi

  request PATCH '/api/v1/me' "$COOKIE" "$WEB_URL" '{"nickname":"Web Updated","avatar_url":"https://example.invalid/avatar.png"}'
  if [ "$STATUS" != 200 ] || ! json '.user.nickname == "Web Updated"'; then
    fail "$N" ProfileUpdate '200 updated profile' "$STATUS $(cat "$BODY")"; return
  fi

  request PATCH '/api/v1/me' "$COOKIE" 'https://evil.example' '{"nickname":"Rejected","avatar_url":""}'
  if [ "$STATUS" != 403 ] || ! json '.error == "csrf_origin_rejected"'; then
    fail "$N" CSRF '403 csrf_origin_rejected' "$STATUS $(cat "$BODY")"; return
  fi

  request POST '/api/v1/me/password' "$COOKIE" "$WEB_URL" "{\"current_password\":\"$P1\",\"new_password\":\"$P2\",\"totp_code\":\"\",\"recovery_code\":\"\"}"
  if [ "$STATUS" != 200 ] || ! json '.status == "password_changed"'; then
    fail "$N" ChangePassword '200 password_changed' "$STATUS $(cat "$BODY")"; return
  fi

  request POST '/api/v1/auth/login' '' "$WEB_URL" "{\"email\":\"$EMAIL\",\"password\":\"$P1\"}"
  [ "$STATUS" = 401 ] || { fail "$N" OldPasswordRejected 401 "$STATUS $(cat "$BODY")"; return; }
  request POST '/api/v1/auth/login' "$COOKIE" "$WEB_URL" "{\"email\":\"$EMAIL\",\"password\":\"$P2\",\"remember_me\":true}"
  [ "$STATUS" = 200 ] || { fail "$N" NewPasswordLogin 200 "$STATUS $(cat "$BODY")"; return; }

  request POST '/api/v1/auth/recovery/request' '' "$WEB_URL" "{\"email\":\"$EMAIL\"}"
  if [ "$STATUS" != 202 ] || ! json '.status == "accepted"'; then
    fail "$N" RecoveryRequest '202 accepted' "$STATUS $(cat "$BODY")"; return
  fi
  TOKEN=$(value '.recovery_token // empty')
  if [ -n "$TOKEN" ]; then
    P3='Web-Recovered-Password-2026!'
    request POST '/api/v1/auth/recovery/reset' '' "$WEB_URL" "{\"token\":\"$TOKEN\",\"new_password\":\"$P3\"}"
    if [ "$STATUS" != 200 ] || ! json '.status == "password_reset"'; then
      fail "$N" RecoveryReset '200 password_reset' "$STATUS $(cat "$BODY")"; return
    fi
    request GET '/api/v1/me' "$COOKIE"
    [ "$STATUS" = 401 ] || { fail "$N" SessionRevocation 401 "$STATUS $(cat "$BODY")"; return; }
  fi
  ok "$N"
}

plain_http_rejected() {
  N=WebPlainHTTPAuthRejected
  case "$WEB_URL" in
    https://localhost:8443|https://127.0.0.1:8443)
      HTTP_URL=$(printf '%s' "$WEB_URL" | sed 's#^https://#http://#; s#:8443#:8081#') ;;
    *) normal "$N"; return ;;
  esac
  STATUS=$(curl -sS -o "$BODY" -w '%{http_code}' -H 'Content-Type: application/json' --data '{"email":"plain@example.invalid","password":"unused"}' "$HTTP_URL/api/v1/auth/login")
  if [ "$STATUS" != 403 ] || ! json '.error == "https_required"'; then
    fail "$N" HTTPSPolicy '403 https_required' "$STATUS $(cat "$BODY")"; return
  fi
  ok "$N"
}

ranking_lists() {
  N=WebRankingLists

  request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=10&page=1&sort=popular'
  if [ "$STATUS" != 200 ] || ! json '(.apps | type == "array" and length <= 10 and length > 0) and ([.apps[].popularity_score] == ([.apps[].popularity_score] | sort | reverse))'; then
    fail "$N" Popular 'top 10 sorted by descending server-side popularity score' "$STATUS $(cat "$BODY")"; return
  fi

  request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=10&page=1&sort=downloads'
  if [ "$STATUS" != 200 ] || ! json '(.apps | type == "array" and length <= 10 and length > 0) and .apps[0].slug == "pixelmator" and .apps[0].downloads >= 50'; then
    fail "$N" Downloads 'top 10 by completed unique download count' "$STATUS $(cat "$BODY")"; return
  fi

  request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=10&page=1&sort=new'
  if [ "$STATUS" != 200 ] || ! json '(.apps | type == "array" and length <= 10 and length > 0) and .apps[0].name == "API Uploaded App" and (.apps | any(.slug == "libreoffice"))'; then
    fail "$N" NewReleases 'top 10 newest catalog uploads with the integration upload first' "$STATUS $(cat "$BODY")"; return
  fi

  ok "$N"
}

admin_upload() {
  N=WebAdminAndUpload
  AE=${API_TEST_ADMIN_EMAIL:-}; AP=${API_TEST_ADMIN_PASSWORD:-}
  if [ -z "$AE" ] || [ -z "$AP" ]; then normal "$N"; return; fi

  request POST '/api/v1/auth/login' "$ADMIN_COOKIE" "$WEB_URL" "{\"email\":\"$AE\",\"password\":\"$AP\",\"remember_me\":true}"
  if [ "$STATUS" != 200 ] || ! json '.user.roles | index("admin")'; then
    fail "$N" AdminLogin '200 admin' "$STATUS $(cat "$BODY")"; return
  fi
  request GET '/api/v1/admin/dashboard' "$ADMIN_COOKIE"
  if [ "$STATUS" != 200 ] || ! json '.dashboard.users >= 1'; then
    fail "$N" Dashboard '200 dashboard' "$STATUS $(cat "$BODY")"; return
  fi

  SUFFIX="$(date +%s)-$$"
  request POST '/api/v1/admin/apps' "$ADMIN_COOKIE" "$WEB_URL" "{\"slug\":\"web-upload-$SUFFIX\",\"name\":\"Web Upload Test\",\"bundle_id\":\"org.legacystore.webtest.$SUFFIX\",\"developer_name\":\"LegacyStore\",\"summary\":\"Web proxy integration test\",\"description\":\"Temporary test record.\",\"category_slug\":\"utilities\"}"
  APP_ID=$(value '.app.id // empty')
  [ "$STATUS" = 200 ] && [ -n "$APP_ID" ] || { fail "$N" CreateApp '200 app id' "$STATUS $(cat "$BODY")"; return; }

  request POST "/api/v1/admin/apps/$APP_ID/versions" "$ADMIN_COOKIE" "$WEB_URL" '{"version":"1.0-web-test","release_date":"2026-09-18","changelog":"Web integration test","is_recommended":false}'
  VERSION_ID=$(value '.version.id // empty')
  [ "$STATUS" = 201 ] && [ -n "$VERSION_ID" ] || { fail "$N" CreateVersion '201 version id' "$STATUS $(cat "$BODY")"; return; }

  FILE="$TMP_DIR/Web-Integration-Test.dmg"
  printf 'LegacyStore web proxy upload integration test\n' > "$FILE"
  STATUS=$(curl -k -sS -o "$BODY" -D "$HEADERS" -w '%{http_code}' -b "$ADMIN_COOKIE" -c "$ADMIN_COOKIE" -H "Origin: $WEB_URL" \
    -F "file=@$FILE" -F 'min_os=10.4' -F 'max_tested_os=10.15' -F 'arch_i386=true' -F 'arch_x86_64=true' -F 'supports_32bit=true' -F 'supports_64bit=true' \
    "$WEB_URL/api/v1/admin/versions/$VERSION_ID/upload")
  if [ "$STATUS" != 200 ] || ! json '.artifact.id and .artifact.moderation_status == "pending" and .upload.status == "quarantined" and (.upload.sha256 | length == 64)'; then
    fail "$N" MultipartUpload 'quarantined artifact with SHA-256' "$STATUS $(cat "$BODY")"; return
  fi
  ok "$N"
}

web_shell
frontend_bundle
proxy_bootstrap
account_lifecycle
plain_http_rejected
ranking_lists
admin_upload

if [ "$FAILED" -eq 0 ]; then echo 'All web integration tests passed.'; else echo 'Web integration tests finished with errors.'; fi
echo 'Summary:'
printf -- '- Passed: %s\n- Normal: %s\n- Failed: %s\n' "$PASSED" "$NORMAL" "$FAILED"
[ "$FAILED" -eq 0 ] || exit 1
