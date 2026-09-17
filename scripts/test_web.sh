#!/bin/sh
set -u

WEB_URL=${WEB_URL:-https://localhost:8443}
TMP_DIR=$(mktemp -d)
BODY_FILE="$TMP_DIR/body"
HEADERS_FILE="$TMP_DIR/headers"
COOKIE_JAR="$TMP_DIR/cookies"
ADMIN_COOKIE_JAR="$TMP_DIR/admin-cookies"
PASSED=0
NORMAL=0
FAILED=0

GREEN=$(printf '\033[32m')
YELLOW=$(printf '\033[33m')
RED=$(printf '\033[31m')
RESET=$(printf '\033[0m')

trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

for tool in curl jq grep; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "$tool is required." >&2
    exit 127
  fi
done

ok() {
  PASSED=$((PASSED + 1))
  printf '%s: %s[OK]%s\n' "$1" "$GREEN" "$RESET"
}

normal() {
  NORMAL=$((NORMAL + 1))
  printf '%s: %s[NORMAL]%s\n' "$1" "$YELLOW" "$RESET"
}

err() {
  FAILED=$((FAILED + 1))
  printf '%s: %s[ERR]%s\n' "$1" "$RED" "$RESET"
  printf '%s caught error %s:\n' "$1" "$2"
  printf '> Expected: %s\n' "$3"
  printf '> Got: %s\n' "$4"
}

request() {
  METHOD=$1
  PATHNAME=$2
  JAR=${3:-}
  ORIGIN=${4:-}
  DATA=${5:-}

  set -- -k -sS -o "$BODY_FILE" -D "$HEADERS_FILE" -w '%{http_code}' -X "$METHOD" -H 'Accept: application/json'
  if [ -n "$JAR" ]; then
    set -- "$@" -b "$JAR" -c "$JAR"
  fi
  if [ -n "$ORIGIN" ]; then
    set -- "$@" -H "Origin: $ORIGIN"
  fi
  if [ -n "$DATA" ]; then
    set -- "$@" -H 'Content-Type: application/json' --data "$DATA"
  fi
  HTTP_STATUS=$(curl "$@" "$WEB_URL$PATHNAME")
}

status_is() {
  [ "$HTTP_STATUS" = "$1" ]
}

json_ok() {
  jq -e "$1" "$BODY_FILE" >/dev/null 2>&1
}

body_value() {
  jq -r "$1" "$BODY_FILE" 2>/dev/null
}

header_contains() {
  grep -Eiq "$1" "$HEADERS_FILE"
}

static_request() {
  PATHNAME=$1
  HTTP_STATUS=$(curl -k -sS -o "$BODY_FILE" -D "$HEADERS_FILE" -w '%{http_code}' "$WEB_URL$PATHNAME")
}

test_web_shell() {
  NAME='WebShell'
  static_request '/'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$HTTP_STATUS"
    return
  fi
  if ! grep -q 'LegacyStore' "$BODY_FILE" || ! grep -q '/app.js' "$BODY_FILE" || ! grep -q '/ui-fixes.css' "$BODY_FILE" || ! grep -q '<svg' "$BODY_FILE"; then
    err "$NAME" HtmlShape 'LegacyStore shell with app.js, ui-fixes.css and SVG toolbar icons' "$(head -c 500 "$BODY_FILE")"
    return
  fi
  if ! header_contains '^Content-Security-Policy:' || ! header_contains '^X-Frame-Options:[[:space:]]*DENY'; then
    err "$NAME" SecurityHeaders 'CSP and X-Frame-Options' "$(cat "$HEADERS_FILE")"
    return
  fi
  ok "$NAME"
}

test_frontend_bundle() {
  NAME='FrontendBundle'
  static_request '/app.js'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$HTTP_STATUS"
    return
  fi
  for marker in '/me/password' '/me/email' '/auth/recovery/request' '/auth/2fa/recovery-codes/regenerate' '/admin/versions/' 'recovery_code' 'app-icon-image'; do
    if ! grep -Fq "$marker" "$BODY_FILE"; then
      err "$NAME" MissingIntegrationMarker "$marker" 'not found'
      return
    fi
  done
  if grep -Fq 'document.cookie' "$BODY_FILE"; then
    err "$NAME" SessionSecurity 'no client-side session cookie writes' 'document.cookie found'
    return
  fi

  static_request '/ui-fixes.css'
  if ! status_is 200 || ! grep -Fq '.app-icon-image' "$BODY_FILE" || ! grep -Fq '.tab-icon' "$BODY_FILE"; then
    err "$NAME" IconStyles 'app icon and SVG toolbar styles' "$(head -c 500 "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_proxy_bootstrap() {
  NAME='WebProxyBootstrap'
  request GET '/api/v1/bootstrap'
  if ! status_is 200 || ! json_ok '.server_status == "ok" and .api_version == "v1"'; then
    err "$NAME" BackendProxy '200 with server_status=ok' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi
  if ! header_contains '^Strict-Transport-Security:'; then
    err "$NAME" HSTS 'Strict-Transport-Security over HTTPS' "$(cat "$HEADERS_FILE")"
    return
  fi
  ok "$NAME"
}

test_account_through_web() {
  NAME='WebAccountLifecycle'
  TEST_EMAIL="web-$(date +%s)-$$@example.invalid"
  TEST_PASSWORD='Web-Test-Password-2026!'
  TEST_PASSWORD_2='Web-Test-Password-Changed-2026!'

  request POST '/api/v1/auth/register' "$COOKIE_JAR" "$WEB_URL" "{\"email\":\"$TEST_EMAIL\",\"nickname\":\"Web Integration\",\"password\":\"$TEST_PASSWORD\",\"remember_me\":true}"
  if ! status_is 200 || ! json_ok --arg email "$TEST_EMAIL" '.user.email == $email'; then
    err "$NAME" Register '200 and matching user' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi
  if ! header_contains '^Set-Cookie:[[:space:]]*legacystore_session=' || ! header_contains 'HttpOnly' || ! header_contains 'Secure' || ! header_contains 'SameSite=Lax'; then
    err "$NAME" SessionCookie 'Secure HttpOnly SameSite=Lax session cookie' "$(cat "$HEADERS_FILE")"
    return
  fi

  request GET '/api/v1/me' "$COOKIE_JAR"
  if ! status_is 200 || ! json_ok --arg email "$TEST_EMAIL" '.user.email == $email and .session.auth_kind == "web"'; then
    err "$NAME" CookieSession 'authenticated web session through nginx' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi

  request PATCH '/api/v1/me' "$COOKIE_JAR" "$WEB_URL" '{"nickname":"Web Updated","avatar_url":"https://example.invalid/avatar.png"}'
  if ! status_is 200 || ! json_ok '.user.nickname == "Web Updated"'; then
    err "$NAME" ProfileUpdate '200 and updated nickname' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi

  request PATCH '/api/v1/me' "$COOKIE_JAR" 'https://evil.example' '{"nickname":"Should Not Apply","avatar_url":""}'
  if ! status_is 403 || ! json_ok '.error == "csrf_origin_rejected"'; then
    err "$NAME" CrossOriginCSRF '403 csrf_origin_rejected' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi

  request POST '/api/v1/me/password' "$COOKIE_JAR" "$WEB_URL" "{\"current_password\":\"$TEST_PASSWORD\",\"new_password\":\"$TEST_PASSWORD_2\",\"totp_code\":\"\",\"recovery_code\":\"\"}"
  if ! status_is 200 || ! json_ok '.status == "password_changed"'; then
    err "$NAME" ChangePassword '200 password_changed' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi

  request POST '/api/v1/auth/login' '' "$WEB_URL" "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASSWORD\"}"
  if ! status_is 401; then
    err "$NAME" OldPasswordRejected 401 "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi

  request POST '/api/v1/auth/login' "$COOKIE_JAR" "$WEB_URL" "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASSWORD_2\",\"remember_me\":true}"
  if ! status_is 200; then
    err "$NAME" NewPasswordLogin 200 "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi

  request POST '/api/v1/auth/recovery/request' '' "$WEB_URL" "{\"email\":\"$TEST_EMAIL\"}"
  if ! status_is 202 || ! json_ok '.status == "accepted"'; then
    err "$NAME" RecoveryRequest '202 accepted' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi
  RECOVERY_TOKEN=$(body_value '.recovery_token // empty')
  if [ -n "$RECOVERY_TOKEN" ]; then
    RESET_PASSWORD='Web-Recovered-Password-2026!'
    request POST '/api/v1/auth/recovery/reset' '' "$WEB_URL" "{\"token\":\"$RECOVERY_TOKEN\",\"new_password\":\"$RESET_PASSWORD\"}"
    if ! status_is 200 || ! json_ok '.status == "password_reset"'; then
      err "$NAME" RecoveryReset '200 password_reset' "$HTTP_STATUS $(cat "$BODY_FILE")"
      return
    fi
    request GET '/api/v1/me' "$COOKIE_JAR"
    if ! status_is 401; then
      err "$NAME" RecoveryRevokesSessions 401 "$HTTP_STATUS $(cat "$BODY_FILE")"
      return
    fi
  fi

  ok "$NAME"
}

test_plain_http_auth_rejected() {
  NAME='WebPlainHTTPAuthRejected'
  case "$WEB_URL" in
    https://localhost:8443|https://127.0.0.1:8443)
      HTTP_URL=$(printf '%s' "$WEB_URL" | sed 's#^https://#http://#; s#:8443#:8081#')
      ;;
    *)
      normal "$NAME"
      return
      ;;
  esac

  STATUS=$(curl -sS -o "$BODY_FILE" -w '%{http_code}' -H 'Content-Type: application/json' --data '{"email":"plain-web@example.invalid","password":"not-used"}' "$HTTP_URL/api/v1/auth/login")
  if [ "$STATUS" != "403" ] || ! json_ok '.error == "https_required"'; then
    err "$NAME" HTTPSPolicy '403 https_required through HTTP web proxy' "$STATUS $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_admin_through_web() {
  NAME='WebAdminAndUpload'
  ADMIN_EMAIL=${API_TEST_ADMIN_EMAIL:-}
  ADMIN_PASSWORD=${API_TEST_ADMIN_PASSWORD:-}
  if [ -z "$ADMIN_EMAIL" ] || [ -z "$ADMIN_PASSWORD" ]; then
    normal "$NAME"
    return
  fi

  request POST '/api/v1/auth/login' "$ADMIN_COOKIE_JAR" "$WEB_URL" "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\",\"remember_me\":true}"
  if ! status_is 200 || ! json_ok '.user.roles | index("admin")'; then
    err "$NAME" AdminLogin '200 admin user' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi

  request GET '/api/v1/admin/dashboard' "$ADMIN_COOKIE_JAR"
  if ! status_is 200 || ! json_ok '.dashboard.users >= 1'; then
    err "$NAME" Dashboard '200 dashboard' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi

  SUFFIX="$(date +%s)-$$"
  APP_SLUG="web-upload-$SUFFIX"
  request POST '/api/v1/admin/apps' "$ADMIN_COOKIE_JAR" "$WEB_URL" "{\"slug\":\"$APP_SLUG\",\"name\":\"Web Upload Test\",\"bundle_id\":\"org.legacystore.webtest.$SUFFIX\",\"developer_name\":\"LegacyStore\",\"summary\":\"Web proxy integration test\",\"description\":\"Temporary test record.\",\"category_slug\":\"utilities\"}"
  APP_ID=$(body_value '.app.id // empty')
  if ! status_is 200 || [ -z "$APP_ID" ]; then
    err "$NAME" CreateApp '200 with app id' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi

  request POST "/api/v1/admin/apps/$APP_ID/versions" "$ADMIN_COOKIE_JAR" "$WEB_URL" '{"version":"1.0-web-test","release_date":"2026-09-18","changelog":"Web integration test","is_recommended":false}'
  VERSION_ID=$(body_value '.version.id // empty')
  if ! status_is 201 || [ -z "$VERSION_ID" ]; then
    err "$NAME" CreateVersion '201 with version id' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi

  UPLOAD_FILE="$TMP_DIR/Web-Integration-Test.dmg"
  printf 'LegacyStore web proxy upload integration test\n' > "$UPLOAD_FILE"
  HTTP_STATUS=$(curl -k -sS -o "$BODY_FILE" -D "$HEADERS_FILE" -w '%{http_code}' \
    -b "$ADMIN_COOKIE_JAR" -c "$ADMIN_COOKIE_JAR" \
    -H "Origin: $WEB_URL" \
    -F "file=@$UPLOAD_FILE" \
    -F 'min_os=10.4' \
    -F 'max_tested_os=10.15' \
    -F 'arch_i386=true' \
    -F 'arch_x86_64=true' \
    -F 'supports_32bit=true' \
    -F 'supports_64bit=true' \
    "$WEB_URL/api/v1/admin/versions/$VERSION_ID/upload")
  if ! status_is 200 || ! json_ok '.artifact.id and .artifact.moderation_status == "pending" and .upload.status == "quarantined" and (.upload.sha256 | length == 64)'; then
    err "$NAME" MultipartUpload 'quarantined artifact with SHA-256 through nginx' "$HTTP_STATUS $(cat "$BODY_FILE")"
    return
  fi

  ok "$NAME"
}

test_web_shell
test_frontend_bundle
test_proxy_bootstrap
test_account_through_web
test_plain_http_auth_rejected
test_admin_through_web

if [ "$FAILED" -eq 0 ]; then
  echo 'All web integration tests passed.'
else
  echo 'Web integration tests finished with errors.'
fi

echo 'Summary:'
printf -- '- Passed: %s\n' "$PASSED"
printf -- '- Normal: %s\n' "$NORMAL"
printf -- '- Failed: %s\n' "$FAILED"

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi
