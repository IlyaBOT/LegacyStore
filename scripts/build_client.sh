#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SRC_DIR="$ROOT_DIR/legacy-client/LegacyStore"
BUILD_ROOT=${LEGACYSTORE_CLIENT_BUILD_DIR:-"$ROOT_DIR/build/legacy-client"}
APP_DIR="$ROOT_DIR/build/LegacyStore.app"

TIGER_SDK=${LEGACYSTORE_TIGER_SDK:-/Developer/SDKs/MacOSX10.4u.sdk}
LEOPARD_SDK=${LEGACYSTORE_LEOPARD_SDK:-/Developer/SDKs/MacOSX10.5.sdk}

find_compiler() {
  if [ -n "${LEGACYSTORE_CLIENT_CC:-}" ]; then
    printf '%s\n' "$LEGACYSTORE_CLIENT_CC"
    return
  fi
  for candidate in gcc-4.2 gcc-4.0 llvm-gcc-4.2 gcc; do
    if command -v "$candidate" >/dev/null 2>&1; then
      command -v "$candidate"
      return
    fi
  done
  return 1
}

CC=$(find_compiler || true)
if [ -z "$CC" ]; then
  echo "No compatible Apple GCC/LLVM-GCC compiler was found." >&2
  echo "Use Xcode 3.x on Leopard/Snow Leopard or set LEGACYSTORE_CLIENT_CC explicitly." >&2
  exit 127
fi

if ! command -v lipo >/dev/null 2>&1; then
  echo "lipo is required to create the universal LegacyStore executable." >&2
  exit 127
fi

if [ ! -d "$TIGER_SDK" ]; then
  echo "Mac OS X 10.4u SDK was not found at: $TIGER_SDK" >&2
  echo "Install the 10.4u SDK with an Xcode 3.x toolchain or set LEGACYSTORE_TIGER_SDK." >&2
  exit 1
fi

if [ ! -d "$LEOPARD_SDK" ]; then
  if [ -d /Developer/SDKs/MacOSX10.6.sdk ]; then
    LEOPARD_SDK=/Developer/SDKs/MacOSX10.6.sdk
    echo "Mac OS X 10.5 SDK not found; using 10.6 SDK for the x86_64 slice with a 10.5 deployment target."
  else
    echo "Mac OS X 10.5/10.6 SDK was not found." >&2
    echo "Set LEGACYSTORE_LEOPARD_SDK to a compatible SDK." >&2
    exit 1
  fi
fi

SOURCES="
$SRC_DIR/main.m
$SRC_DIR/LSAppDelegate.m
$SRC_DIR/LSMainWindowController.m
$SRC_DIR/LSSystemInfo.m
$SRC_DIR/LSJSONParser.m
$SRC_DIR/LSAPIClient.m
"

for source in $SOURCES; do
  if [ ! -f "$source" ]; then
    echo "Missing client source file: $source" >&2
    exit 1
  fi
done

rm -rf "$BUILD_ROOT" "$APP_DIR"
mkdir -p "$BUILD_ROOT/i386" "$BUILD_ROOT/x86_64" "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"

COMMON_FLAGS="-Wall -Wextra -fobjc-exceptions -framework Cocoa"

echo "Compiler: $CC"
echo "Tiger SDK: $TIGER_SDK"
echo "Leopard+ SDK: $LEOPARD_SDK"
echo "Building i386 slice for Mac OS X 10.4..."
MACOSX_DEPLOYMENT_TARGET=10.4 "$CC" \
  -arch i386 \
  -isysroot "$TIGER_SDK" \
  -mmacosx-version-min=10.4 \
  $COMMON_FLAGS \
  $SOURCES \
  -o "$BUILD_ROOT/i386/LegacyStore"

echo "Building x86_64 slice for Mac OS X 10.5+..."
MACOSX_DEPLOYMENT_TARGET=10.5 "$CC" \
  -arch x86_64 \
  -isysroot "$LEOPARD_SDK" \
  -mmacosx-version-min=10.5 \
  $COMMON_FLAGS \
  $SOURCES \
  -o "$BUILD_ROOT/x86_64/LegacyStore"

echo "Creating universal executable..."
lipo -create \
  "$BUILD_ROOT/i386/LegacyStore" \
  "$BUILD_ROOT/x86_64/LegacyStore" \
  -output "$APP_DIR/Contents/MacOS/LegacyStore"

cp "$SRC_DIR/Info.plist" "$APP_DIR/Contents/Info.plist"
chmod 755 "$APP_DIR/Contents/MacOS/LegacyStore"

echo "Built: $APP_DIR"
echo "Architectures:"
lipo -info "$APP_DIR/Contents/MacOS/LegacyStore"
