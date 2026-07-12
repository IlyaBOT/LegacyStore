#!/bin/sh
set -u

BASE_URL=${BASE_URL:-http://localhost:8080}

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required." >&2
  exit 127
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required." >&2
  exit 127
fi

TMP_DIR=$(mktemp -d)
BODY_FILE="$TMP_DIR/body.json"
STATUS_FILE="$TMP_DIR/status.txt"

PASSED=0
NORMAL=0
FAILED=0

GREEN=$(printf '\033[32m')
YELLOW=$(printf '\033[33m')
RED=$(printf '\033[31m')
RESET=$(printf '\033[0m')

trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

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
  PATH_INFO=$2
  DATA=${3:-}

  if [ "$METHOD" = "POST" ]; then
    HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -w '%{http_code}' \
      -X POST \
      -H 'Content-Type: application/json' \
      --data "$DATA" \
      "$BASE_URL$PATH_INFO")
  else
    HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -w '%{http_code}' "$BASE_URL$PATH_INFO")
  fi

  printf '%s' "$HTTP_STATUS" > "$STATUS_FILE"
}

request_secure() {
  METHOD=$1
  PATH_INFO=$2
  DATA=${3:-}

  if [ "$METHOD" = "POST" ]; then
    HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -w '%{http_code}' \
      -X POST \
      -H 'Content-Type: application/json' \
      -H 'X-Forwarded-Proto: https' \
      --data "$DATA" \
      "$BASE_URL$PATH_INFO")
  else
    HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -w '%{http_code}' \
      -H 'X-Forwarded-Proto: https' \
      "$BASE_URL$PATH_INFO")
  fi

  printf '%s' "$HTTP_STATUS" > "$STATUS_FILE"
}

secure_json() {
  METHOD=$1
  PATH_INFO=$2
  COOKIE_FILE=$3
  DATA=${4:-}

  if [ -n "$COOKIE_FILE" ] && [ -n "$DATA" ]; then
    HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -w '%{http_code}' \
      -b "$COOKIE_FILE" \
      -c "$COOKIE_FILE" \
      -X "$METHOD" \
      -H 'Content-Type: application/json' \
      -H 'X-Forwarded-Proto: https' \
      --data "$DATA" \
      "$BASE_URL$PATH_INFO")
  elif [ -n "$COOKIE_FILE" ]; then
    HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -w '%{http_code}' \
      -b "$COOKIE_FILE" \
      -c "$COOKIE_FILE" \
      -X "$METHOD" \
      -H 'X-Forwarded-Proto: https' \
      "$BASE_URL$PATH_INFO")
  elif [ -n "$DATA" ]; then
    HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -w '%{http_code}' \
      -X "$METHOD" \
      -H 'Content-Type: application/json' \
      -H 'X-Forwarded-Proto: https' \
      --data "$DATA" \
      "$BASE_URL$PATH_INFO")
  else
    HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -w '%{http_code}' \
      -X "$METHOD" \
      -H 'X-Forwarded-Proto: https' \
      "$BASE_URL$PATH_INFO")
  fi

  printf '%s' "$HTTP_STATUS" > "$STATUS_FILE"
}

status_is() {
  EXPECTED=$1
  ACTUAL=$(cat "$STATUS_FILE")
  [ "$ACTUAL" = "$EXPECTED" ]
}

status_is_one_of() {
  FIRST=$1
  SECOND=$2
  ACTUAL=$(cat "$STATUS_FILE")
  [ "$ACTUAL" = "$FIRST" ] || [ "$ACTUAL" = "$SECOND" ]
}

jq_ok() {
  jq -e "$1" "$BODY_FILE" >/dev/null 2>&1
}

body_value() {
  jq -r "$1" "$BODY_FILE" 2>/dev/null
}

totp_code() {
  SECRET=$1
  python3 - "$SECRET" <<'PY'
import base64
import hashlib
import hmac
import struct
import sys
import time

secret = sys.argv[1].strip().replace(" ", "")
secret += "=" * ((8 - len(secret) % 8) % 8)
key = base64.b32decode(secret, casefold=True)
counter = int(time.time()) // 30
digest = hmac.new(key, struct.pack(">Q", counter), hashlib.sha1).digest()
offset = digest[-1] & 0x0F
value = struct.unpack(">I", digest[offset:offset + 4])[0] & 0x7FFFFFFF
print(f"{value % 1000000:06d}")
PY
}

register_cookie() {
  PREFIX=$1
  COOKIE_FILE=$2
  TEST_EMAIL="$PREFIX-$(date +%s)-$$-$PASSED@example.invalid"
  TEST_PASSWORD='correct horse battery staple'
  secure_json POST '/api/v1/auth/register' "$COOKIE_FILE" "{\"email\":\"$TEST_EMAIL\",\"nickname\":\"$PREFIX\",\"password\":\"$TEST_PASSWORD\",\"remember_me\":true}"
}

test_bootstrap() {
  NAME='Bootstrap'
  request GET '/api/v1/bootstrap'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.catalog_format_version and .api_version and .server_status == "ok"'; then
    err "$NAME" JsonShape 'api_version, catalog_format_version, server_status=ok' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_categories() {
  NAME='Categories'
  request GET '/api/v1/categories'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.categories | type == "array" and length > 0 and .[0].slug and .[0].name'; then
    err "$NAME" JsonShape 'non-empty categories with slug and name' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_app_list() {
  NAME='AppList'
  request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=12&page=1'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.apps | type == "array" and length > 0 and .[0].slug and .[0].name and .[0].compatibility'; then
    err "$NAME" JsonShape 'non-empty apps with slug, name, compatibility' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_app_list_category_filter() {
  NAME='AppListCategoryFilter'
  request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=12&page=1&category=graphics-design'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.apps | type == "array" and length > 0 and all(.category == "Graphics & Design")'; then
    err "$NAME" CategoryFilter 'only Graphics & Design apps' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_app_list_pagination() {
  NAME='AppListPagination'
  request GET '/api/v1/apps?os=10.9.5&arch=x86_64&limit=3&page=1'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.page == 1 and .limit == 3 and (.apps | length <= 3)'; then
    err "$NAME" Pagination 'page=1, limit=3, at most 3 apps' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_search() {
  NAME='Search'
  request GET '/api/v1/search?q=vlc&os=10.9.5&arch=x86_64'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.results | type == "array"'; then
    err "$NAME" JsonShape 'results array' "$(cat "$BODY_FILE")"
    return
  fi
  if jq_ok '.results | any(.slug == "vlc")'; then
    ok "$NAME"
  else
    err "$NAME" ExpectedData 'VLC result' "$(cat "$BODY_FILE")"
  fi
}

test_app_detail() {
  NAME='AppDetail'
  request GET '/api/v1/apps/pixelmator?os=10.9.5&arch=x86_64'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.slug and .name and (.versions or .recommended_artifact) and .compatibility'; then
    err "$NAME" JsonShape 'slug, name, versions or recommended_artifact, compatibility' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_versions() {
  NAME='AppVersions'
  request GET '/api/v1/apps/pixelmator/versions?os=10.9.5&arch=x86_64'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.versions | type == "array" and length > 0 and .[0].version and (.[0].artifacts | type == "array") and (.[0].artifacts[0].compatibility_status | type == "string")'; then
    err "$NAME" JsonShape 'versions with artifacts and compatibility_status' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_os_too_old() {
  NAME='CompatibilityOsTooOld'
  request GET '/api/v1/apps/pixelmator?os=10.5.8&arch=i386'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.compatibility'; then
    err "$NAME" JsonShape 'compatibility object' "$(cat "$BODY_FILE")"
    return
  fi
  if jq_ok '.compatibility.status == "blocked"'; then
    if jq_ok '.compatibility.reasons | length > 0'; then
      ok "$NAME"
    else
      err "$NAME" CompatibilityReasons 'blocked response with reasons' "$(cat "$BODY_FILE")"
    fi
  else
    normal "$NAME"
  fi
}

test_catalina_32bit() {
  NAME='CompatibilityCatalina32Bit'
  request GET '/api/v1/apps/legacy-32bit-test?os=10.15&arch=x86_64'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.compatibility.status == "blocked" and (.compatibility.reasons | any(.code == "bitness_mismatch" or .code == "requires_32bit"))'; then
    err "$NAME" Compatibility 'blocked with bitness_mismatch or requires_32bit' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_compatible_only_filter() {
  NAME='CompatibleOnlyFilter'
  request GET '/api/v1/apps?os=10.15&arch=x86_64&compatible=1&limit=50&page=1'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.apps | type == "array" and all(.compatibility.status != "blocked") and all(.slug != "legacy-32bit-test")'; then
    err "$NAME" CompatibilityFilter 'only non-blocked apps for macOS 10.15' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_download_metadata() {
  NAME='DownloadMetadata'
  request GET '/api/v1/apps/pixelmator?os=10.9.5&arch=x86_64'
  ARTIFACT_ID=$(body_value '.recommended_artifact.id')
  if [ -z "$ARTIFACT_ID" ] || [ "$ARTIFACT_ID" = "null" ]; then
    err "$NAME" ExpectedData 'recommended_artifact.id' "$(cat "$BODY_FILE")"
    return
  fi

  request GET "/api/v1/download/$ARTIFACT_ID"
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '(.download_url or .external_page_url or .source_type) and .sha256 and .size_bytes'; then
    err "$NAME" JsonShape 'download source, sha256, size_bytes' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_unknown_download() {
  NAME='UnknownDownload'
  request GET '/api/v1/download/999999'
  if ! status_is 404; then
    err "$NAME" HttpStatus 404 "$(cat "$STATUS_FILE")"
    return
  fi
  ok "$NAME"
}

test_invalid_login() {
  NAME='InvalidLogin'
  request POST '/api/v1/auth/login' '{"email":"invalid@example.com","password":"wrong"}'
  if ! status_is_one_of 401 403; then
    err "$NAME" HttpStatus '401 or 403' "$(cat "$STATUS_FILE")"
    return
  fi
  if jq_ok '.token'; then
    err "$NAME" Security 'no token in invalid login response' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_register_requires_https() {
  NAME='RegisterRequiresHttps'
  request POST '/api/v1/auth/register' '{"email":"new@example.com","nickname":"new","password":"wrong"}'
  if ! status_is 403; then
    err "$NAME" HttpStatus 403 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.error == "https_required"'; then
    err "$NAME" Security 'https_required error' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_protected_me() {
  NAME='ProtectedMeRequiresHttps'
  request GET '/api/v1/me'
  if ! status_is 403; then
    err "$NAME" HttpStatus 403 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.error == "https_required"'; then
    err "$NAME" Security 'https_required error' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_protected_me_without_token() {
  NAME='ProtectedMeWithoutToken'
  request_secure GET '/api/v1/me'
  if ! status_is 401; then
    err "$NAME" HttpStatus 401 "$(cat "$STATUS_FILE")"
    return
  fi
  if jq_ok '.email or .profile'; then
    err "$NAME" Security 'no profile data without token' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_register_login_session() {
  NAME='RegisterLoginSession'
  EMAIL="api-$(date +%s)-$$@example.invalid"
  PASSWORD='correct horse battery staple'
  COOKIE_FILE="$TMP_DIR/session.cookies"
  HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -w '%{http_code}' \
    -c "$COOKIE_FILE" \
    -X POST \
    -H 'Content-Type: application/json' \
    -H 'X-Forwarded-Proto: https' \
    --data "{\"email\":\"$EMAIL\",\"nickname\":\"api\",\"password\":\"$PASSWORD\",\"remember_me\":true}" \
    "$BASE_URL/api/v1/auth/register")
  printf '%s' "$HTTP_STATUS" > "$STATUS_FILE"
  if ! status_is 200; then
    err "$NAME" RegisterStatus 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  if ! jq_ok '.user.email and .session_token'; then
    err "$NAME" RegisterShape 'user.email and session_token' "$(cat "$BODY_FILE")"
    return
  fi

  HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -w '%{http_code}' \
    -b "$COOKIE_FILE" \
    -H 'X-Forwarded-Proto: https' \
    "$BASE_URL/api/v1/me")
  printf '%s' "$HTTP_STATUS" > "$STATUS_FILE"
  if ! status_is 200; then
    err "$NAME" MeStatus 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ME_EMAIL=$(body_value '.user.email')
  if [ "$ME_EMAIL" != "$EMAIL" ] || ! jq_ok '.user.roles | type == "array"'; then
    err "$NAME" MeShape 'registered user from cookie session' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_profile_sessions_2fa() {
  NAME='ProfileSessions2FA'
  if ! command -v python3 >/dev/null 2>&1; then
    normal "$NAME"
    return
  fi
  COOKIE_FILE="$TMP_DIR/profile-session.cookies"
  register_cookie profile "$COOKIE_FILE"
  if ! status_is 200; then
    err "$NAME" RegisterStatus 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  EMAIL="$TEST_EMAIL"
  PASSWORD="$TEST_PASSWORD"

  secure_json PATCH '/api/v1/me' "$COOKIE_FILE" '{"nickname":"Profile API"}'
  if ! status_is 200 || ! jq_ok '.user.nickname == "Profile API"'; then
    err "$NAME" ProfileUpdate 'updated nickname' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json GET '/api/v1/me/sessions' "$COOKIE_FILE"
  if ! status_is 200 || ! jq_ok '.sessions | type == "array" and length > 0'; then
    err "$NAME" Sessions 'active sessions array' "$(cat "$BODY_FILE")"
    return
  fi

  secure_json POST '/api/v1/auth/2fa/setup' "$COOKIE_FILE" '{}'
  if ! status_is 200 || ! jq_ok '.secret and .otpauth_url'; then
    err "$NAME" Setup2FA 'secret and otpauth_url' "$(cat "$BODY_FILE")"
    return
  fi
  SECRET=$(body_value '.secret')
  CODE=$(totp_code "$SECRET")

  secure_json POST '/api/v1/auth/2fa/verify' "$COOKIE_FILE" "{\"totp_code\":\"$CODE\"}"
  if ! status_is 200 || ! jq_ok '.status == "enabled"'; then
    err "$NAME" Verify2FA 'enabled' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  LOGIN_COOKIE="$TMP_DIR/profile-login.cookies"
  secure_json POST '/api/v1/auth/login' "$LOGIN_COOKIE" "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
  if ! status_is 401 || ! jq_ok '.error == "two_factor_required"'; then
    err "$NAME" LoginRequires2FA 'two_factor_required without TOTP' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  CODE=$(totp_code "$SECRET")
  secure_json POST '/api/v1/auth/login' "$LOGIN_COOKIE" "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"totp_code\":\"$CODE\"}"
  if ! status_is 200 || ! jq_ok '.user.email == "'"$EMAIL"'" and .session_token'; then
    err "$NAME" LoginWith2FA 'login succeeds with TOTP' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  CODE=$(totp_code "$SECRET")
  secure_json POST '/api/v1/auth/2fa/disable' "$COOKIE_FILE" "{\"totp_code\":\"$CODE\"}"
  if ! status_is 200 || ! jq_ok '.status == "disabled"'; then
    err "$NAME" Disable2FA 'disabled status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  LOGIN_COOKIE="$TMP_DIR/profile-login-after-disable.cookies"
  secure_json POST '/api/v1/auth/login' "$LOGIN_COOKIE" "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"
  if ! status_is 200 || ! jq_ok '.user.email == "'"$EMAIL"'" and (.user.two_factor_enabled == false)'; then
    err "$NAME" LoginAfterDisable 'login succeeds without TOTP after disable' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_legacy_passwords_devices() {
  NAME='LegacyPasswordsDevices'
  COOKIE_FILE="$TMP_DIR/legacy-owner.cookies"
  LEGACY_COOKIE="$TMP_DIR/legacy-client.cookies"
  register_cookie legacy "$COOKIE_FILE"
  if ! status_is 200; then
    err "$NAME" RegisterStatus 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  EMAIL="$TEST_EMAIL"

  secure_json POST '/api/v1/me/legacy-passwords' "$COOKIE_FILE" '{"name":"API Legacy Mac","scopes":["catalog:read","downloads:read"]}'
  if ! status_is 200 || ! jq_ok '.legacy_password.id and .token'; then
    err "$NAME" CreateToken 'legacy_password.id and one-time token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  LEGACY_ID=$(body_value '.legacy_password.id')
  LEGACY_TOKEN=$(body_value '.token')

  secure_json POST '/api/v1/auth/legacy/login' "$LEGACY_COOKIE" "{\"email\":\"$EMAIL\",\"password\":\"$LEGACY_TOKEN\",\"device_identifier\":\"api-device-$$\",\"device_name\":\"API Mac\"}"
  if ! status_is 200 || ! jq_ok '.user.email == "'"$EMAIL"'" and .session_token'; then
    err "$NAME" LegacyLogin 'legacy token login with session' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json GET '/api/v1/me/devices' "$COOKIE_FILE"
  if ! status_is 200 || ! jq_ok '.devices | type == "array" and any(.device_identifier == "api-device-'"$$"'")'; then
    err "$NAME" Devices 'authorized legacy device is listed' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json POST "/api/v1/me/legacy-passwords/$LEGACY_ID/reset" "$COOKIE_FILE" '{}'
  if ! status_is 200 || ! jq_ok '.legacy_password.id and .token'; then
    err "$NAME" ResetToken 'reset returns new one-time token' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json DELETE "/api/v1/me/legacy-passwords/$LEGACY_ID" "$COOKIE_FILE"
  if ! status_is 200 || ! jq_ok '.status == "revoked"'; then
    err "$NAME" RevokeToken 'revoked status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_reviews_write_flow() {
  NAME='ReviewsWriteFlow'
  COOKIE_FILE="$TMP_DIR/review-user.cookies"
  register_cookie review "$COOKIE_FILE"
  if ! status_is 200; then
    err "$NAME" RegisterStatus 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json POST '/api/v1/apps/vlc/reviews' "$COOKIE_FILE" '{"rating":4,"title":"API Review","body":"Created by API smoke test."}'
  if ! status_is 200 || ! jq_ok '.review.id and .review.rating == 4'; then
    err "$NAME" CreateReview 'review id and rating=4' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  REVIEW_ID=$(body_value '.review.id')

  secure_json PATCH "/api/v1/reviews/$REVIEW_ID" "$COOKIE_FILE" '{"rating":5,"title":"API Review Updated","body":"Updated by API smoke test."}'
  if ! status_is 200 || ! jq_ok '.review.rating == 5 and .review.title == "API Review Updated"'; then
    err "$NAME" UpdateReview 'rating=5 and updated title' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json POST "/api/v1/reviews/$REVIEW_ID/like" "$COOKIE_FILE" '{}'
  if ! status_is 200 || ! jq_ok '.status == "liked"'; then
    err "$NAME" LikeReview 'liked status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json POST "/api/v1/reviews/$REVIEW_ID/replies" "$COOKIE_FILE" '{"body":"Reply from API smoke test."}'
  if ! status_is 200 || ! jq_ok '.reply.id and .reply.review_id == '"$REVIEW_ID"; then
    err "$NAME" ReplyReview 'reply id and review_id' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json DELETE "/api/v1/reviews/$REVIEW_ID/like" "$COOKIE_FILE"
  if ! status_is 200 || ! jq_ok '.status == "unliked"'; then
    err "$NAME" UnlikeReview 'unliked status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json DELETE "/api/v1/reviews/$REVIEW_ID" "$COOKIE_FILE"
  if ! status_is 200 || ! jq_ok '.status == "deleted"'; then
    err "$NAME" DeleteReview 'deleted status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_reviews() {
  NAME='ReviewsList'
  request GET '/api/v1/apps/pixelmator/reviews'
  if ! status_is 200; then
    err "$NAME" HttpStatus 200 "$(cat "$STATUS_FILE")"
    return
  fi
  if ! jq_ok '.reviews | type == "array"'; then
    err "$NAME" JsonShape 'reviews array' "$(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_admin_moderation_flow() {
  NAME='AdminModerationFlow'
  ADMIN_COOKIE="$TMP_DIR/admin.cookies"
  UPLOADER_COOKIE="$TMP_DIR/uploader.cookies"
  register_cookie uploader "$UPLOADER_COOKIE"
  if ! status_is 200; then
    err "$NAME" RegisterUploader 200 "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  UPLOADER_ID=$(body_value '.user.id')

  secure_json POST '/api/v1/auth/login' "$ADMIN_COOKIE" '{"email":"admin@legacystore.local","password":"admin12345","remember_me":true}'
  if ! status_is 200 || ! jq_ok '.user.roles | index("admin")'; then
    err "$NAME" AdminLogin 'seed admin login' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json GET '/api/v1/admin/dashboard' "$ADMIN_COOKIE"
  if ! status_is 200 || ! jq_ok '.dashboard.users and .dashboard.apps_approved'; then
    err "$NAME" Dashboard 'dashboard counters' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json GET '/api/v1/admin/users?limit=10' "$ADMIN_COOKIE"
  if ! status_is 200 || ! jq_ok '.users | type == "array" and length > 0'; then
    err "$NAME" Users 'users array' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json POST "/api/v1/admin/users/$UPLOADER_ID/roles" "$ADMIN_COOKIE" '{"role":"trusted"}'
  if ! status_is 200 || ! jq_ok '.status == "ok"'; then
    err "$NAME" AddTrustedRole 'role added' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  APP_SLUG="api-upload-$(date +%s)-$$"
  BUNDLE_ID="com.legacy.apiupload.$(date +%s).$$"
  secure_json POST '/api/v1/admin/apps' "$UPLOADER_COOKIE" "{\"slug\":\"$APP_SLUG\",\"name\":\"API Uploaded App\",\"bundle_id\":\"$BUNDLE_ID\",\"developer_name\":\"LegacyStore\",\"summary\":\"API moderation smoke test\",\"description\":\"Temporary app for moderation smoke test.\",\"category_slug\":\"utilities\"}"
  if ! status_is 200 || ! jq_ok '.app.id and .app.moderation_status == "pending"'; then
    err "$NAME" CreatePendingApp 'trusted upload creates pending app' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  APP_ID=$(body_value '.app.id')

  secure_json GET '/api/v1/admin/moderation?status=pending' "$ADMIN_COOKIE"
  QUEUE_ID=$(jq -r --arg id "$APP_ID" '.items[] | select(.entity_type == "app" and .entity_id == $id) | .id' "$BODY_FILE" 2>/dev/null | head -n 1)
  if ! status_is 200 || [ -z "$QUEUE_ID" ]; then
    err "$NAME" PendingQueue 'pending moderation queue item for uploaded app' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json POST "/api/v1/admin/moderation/$QUEUE_ID/approve" "$ADMIN_COOKIE" '{"comment":"API approved"}'
  if ! status_is 200 || ! jq_ok '.status == "ok"'; then
    err "$NAME" ApproveQueue 'approved status' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi

  secure_json GET '/api/v1/admin/audit-log' "$ADMIN_COOKIE"
  if ! status_is 200 || ! jq_ok '.items | type == "array" and length > 0'; then
    err "$NAME" AuditLog 'audit log array' "$(cat "$STATUS_FILE") $(cat "$BODY_FILE")"
    return
  fi
  ok "$NAME"
}

test_unknown_app() {
  NAME='UnknownApp'
  request GET '/api/v1/apps/this-app-does-not-exist'
  if ! status_is 404; then
    err "$NAME" HttpStatus 404 "$(cat "$STATUS_FILE")"
    return
  fi
  ok "$NAME"
}

test_bootstrap
test_categories
test_app_list
test_app_list_category_filter
test_app_list_pagination
test_search
test_app_detail
test_versions
test_os_too_old
test_catalina_32bit
test_compatible_only_filter
test_download_metadata
test_unknown_download
test_invalid_login
test_register_requires_https
test_protected_me
test_protected_me_without_token
test_register_login_session
test_profile_sessions_2fa
test_legacy_passwords_devices
test_reviews_write_flow
test_reviews
test_admin_moderation_flow
test_unknown_app

if [ "$FAILED" -eq 0 ]; then
  echo 'All API tests passed.'
else
  echo 'API tests finished with errors.'
fi

echo 'Summary:'
printf -- '- Passed: %s\n' "$PASSED"
printf -- '- Normal: %s\n' "$NORMAL"
printf -- '- Failed: %s\n' "$FAILED"

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi
