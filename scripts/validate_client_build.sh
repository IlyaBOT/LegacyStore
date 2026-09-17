#!/bin/sh
set -u

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
APP_DIR=${LEGACYSTORE_CLIENT_APP:-"$ROOT_DIR/build/LegacyStore.app"}
EXECUTABLE="$APP_DIR/Contents/MacOS/LegacyStore"
INFO_PLIST="$APP_DIR/Contents/Info.plist"
TMP_DIR=${TMPDIR:-/tmp}/legacystore-client-validate-$$

PASS=0
FAIL=0

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

ok() {
  PASS=$((PASS + 1))
  printf '%s: [OK]\n' "$1"
}

fail() {
  FAIL=$((FAIL + 1))
  printf '%s: [ERR] %s\n' "$1" "$2" >&2
}

info() {
  printf '%s: [INFO] %s\n' "$1" "$2"
}

printf 'LegacyStore native client validation\n'
printf 'Host: '
sw_vers 2>/dev/null | tr '\n' ' ' || uname -a
printf '\n'

if command -v xcodebuild >/dev/null 2>&1; then
  info Xcode "$(xcodebuild -version 2>/dev/null | tr '\n' ' ')"
else
  fail Xcode 'xcodebuild not found'
fi

for tool in lipo file otool plutil; do
  if command -v "$tool" >/dev/null 2>&1; then
    ok "Tool $tool"
  else
    fail "Tool $tool" 'not found'
  fi
done

if [ ! -d "$APP_DIR" ]; then
  fail AppBundle "missing: $APP_DIR"
elif [ ! -f "$EXECUTABLE" ]; then
  fail Executable "missing: $EXECUTABLE"
else
  ok AppBundle
fi

if [ -f "$INFO_PLIST" ]; then
  if plutil -lint "$INFO_PLIST" >/dev/null 2>&1; then
    ok InfoPlist
  else
    fail InfoPlist 'plutil validation failed'
  fi
else
  fail InfoPlist "missing: $INFO_PLIST"
fi

if [ -x "$EXECUTABLE" ] && command -v lipo >/dev/null 2>&1; then
  LIPO_INFO=$(lipo -info "$EXECUTABLE" 2>&1)
  info Architectures "$LIPO_INFO"
  case "$LIPO_INFO" in
    *i386*) ok I386Slice ;;
    *) fail I386Slice 'i386 architecture is missing' ;;
  esac
  case "$LIPO_INFO" in
    *x86_64*) ok X8664Slice ;;
    *) fail X8664Slice 'x86_64 architecture is missing' ;;
  esac

  mkdir -p "$TMP_DIR"
  if lipo "$EXECUTABLE" -thin i386 -output "$TMP_DIR/LegacyStore-i386" >/dev/null 2>&1; then
    ok ThinI386
    info I386File "$(file "$TMP_DIR/LegacyStore-i386")"
  else
    fail ThinI386 'lipo could not extract i386 slice'
  fi
  if lipo "$EXECUTABLE" -thin x86_64 -output "$TMP_DIR/LegacyStore-x86_64" >/dev/null 2>&1; then
    ok ThinX8664
    info X8664File "$(file "$TMP_DIR/LegacyStore-x86_64")"
  else
    fail ThinX8664 'lipo could not extract x86_64 slice'
  fi

  info LinkedFrameworks "$(otool -L "$EXECUTABLE" 2>/dev/null | tail -n +2 | tr '\n' ';')"
  if otool -l "$EXECUTABLE" 2>/dev/null | grep -A4 LC_VERSION_MIN_MACOSX >/dev/null 2>&1; then
    printf 'Deployment load commands:\n'
    otool -l "$EXECUTABLE" | grep -A4 LC_VERSION_MIN_MACOSX
  else
    info DeploymentLoadCommand 'LC_VERSION_MIN_MACOSX was not reported by this toolchain; verify deployment targets from the build log'
  fi
fi

printf '\nSummary: %d passed, %d failed.\n' "$PASS" "$FAIL"

if [ "$FAIL" -ne 0 ]; then
  exit 1
fi

printf '\nBinary validation passed. Runtime checks are still required:\n'
printf '  arch -i386 "%s"\n' "$EXECUTABLE"
printf '  arch -x86_64 "%s"\n' "$EXECUTABLE"
