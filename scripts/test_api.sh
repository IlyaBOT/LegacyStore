#!/bin/sh
set -u

BASE_URL=${BASE_URL:-http://localhost:8080}
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

for tool in curl jq; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "$tool is required." >&2
    exit 127
  fi
done

TMP_DIR=$(mktemp -d)
BODY_FILE="$TMP_DIR/body.json"
STATUS_FILE="$TMP_DIR/status.txt"
HEADERS_FILE="$TMP_DIR/headers.txt"

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

api_request() {
  AR_METHOD=$1
  AR_PATH=$2
  AR_SECURE=${3:-0}
  AR_TOKEN=${4:-}
  AR_DATA=${5:-}
  AR_AGENT=${6:-LegacyStore-API-Tests/1.0}

  if [ "$AR_SECURE" = "1" ]; then
    AR_PROTO=https
  else
    AR_PROTO=http
  fi

  if [ -n "$AR_DATA" ]; then
    HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -D "$HEADERS_FILE" -w '%{http_code}' \
      -X "$AR_METHOD" \
      -H 'Content-Type: application/json' \
      -H "X-Forwarded-Proto: $AR_PROTO" \
      -H "Authorization: Bearer $AR_TOKEN" \
      -H "User-Agent: $AR_AGENT" \
      --data "$AR_DATA" \
      "$BASE_URL$AR_PATH")
  else
    HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -D "$HEADERS_FILE" -w '%{http_code}' \
      -X "$AR_METHOD" \
      -H "X-Forwarded-Proto: $AR_PROTO" \
      -H "Authorization: Bearer $AR_TOKEN" \
      -H "User-Agent: $AR_AGENT" \
      "$BASE_URL$AR_PATH")
  fi

  printf '%s' "$HTTP_STATUS" > "$STATUS_FILE"
}

make_test_png() {
  MTP_PATH=$1
  MTP_WIDTH=$2
  MTP_HEIGHT=$3
  command -v python3 >/dev/null 2>&1 || return 127
  python3 - "$MTP_PATH" "$MTP_WIDTH" "$MTP_HEIGHT" <<'PY'
import struct
import sys
import zlib

path = sys.argv[1]
width = int(sys.argv[2])
height = int(sys.argv[3])

def chunk(kind, data):
    return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data) & 0xffffffff)

row = bytes([0]) + bytes([40, 120, 220, 255]) * width
raw = row * height
png = b"\x89PNG\r\n\x1a\n"
png += chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0))
png += chunk(b"IDAT", zlib.compress(raw, 9))
png += chunk(b"IEND", b"")
with open(path, "wb") as fh:
    fh.write(png)
PY
}

image_upload_request() {
  IUR_PATH=$1
  IUR_TOKEN=$2
  shift 2
  HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -D "$HEADERS_FILE" -w '%{http_code}'     -X POST     -H 'X-Forwarded-Proto: https'     -H "Authorization: Bearer $IUR_TOKEN"     -H 'User-Agent: LegacyStore-API-Tests/1.0'     "$@"     "$BASE_URL$IUR_PATH")
  printf '%s' "$HTTP_STATUS" > "$STATUS_FILE"
}

multipart_test_upload() {
  MU_PATH=$1
  MU_TOKEN=$2
  MU_FILE=$3
  HTTP_STATUS=$(curl -sS -o "$BODY_FILE" -D "$HEADERS_FILE" -w '%{http_code}' \
    -X POST \
    -H 'X-Forwarded-Proto: https' \
    -H "Authorization: Bearer $MU_TOKEN" \
    -F "file=@$MU_FILE" \
    -F 'min_os=10.4' \
    -F 'max_tested_os=10.15' \
    -F 'arch_i386=true' \
    -F 'arch_x86_64=true' \
    -F 'supports_32bit=true' \
    -F 'supports_64bit=true' \
    "$BASE_URL$MU_PATH")
  printf '%s' "$HTTP_STATUS" > "$STATUS_FILE"
}

status_is() {
  [ "$(cat "$STATUS_FILE")" = "$1" ]
}

jq_ok() {
  jq -e "$1" "$BODY_FILE" >/dev/null 2>&1
}

body_value() {
  jq -r "$1" "$BODY_FILE" 2>/dev/null
}

header_has() {
  grep -Eiq "^$1:[[:space:]]*$2[[:space:]]*$" "$HEADERS_FILE"
}

register_user() {
  RU_PREFIX=$1
  REGISTER_EMAIL="$RU_PREFIX-$(date +%s)-$$-$PASSED@example.invalid"
  REGISTER_PASSWORD='correct horse battery staple'
  api_request POST '/api/v1/auth/register' 1 '' "{\"email\":\"$REGISTER_EMAIL\",\"nickname\":\"$RU_PREFIX\",\"password\":\"$REGISTER_PASSWORD\",\"remember_me\":true}"
  REGISTER_TOKEN=$(body_value '.session_token // empty')
}

totp_code() {
  TC_SECRET=$1
  python3 - "$TC_SECRET" <<'PY'
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

. "$ROOT_DIR/scripts/api_tests/public.sh"
. "$ROOT_DIR/scripts/api_tests/auth.sh"
. "$ROOT_DIR/scripts/api_tests/security_v2.sh"
. "$ROOT_DIR/scripts/api_tests/admin.sh"

run_public_tests
run_auth_tests
run_security_v2_tests
run_admin_tests

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
