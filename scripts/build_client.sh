#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PROJECT_PATH="$ROOT_DIR/legacy-client/LegacyStore.xcodeproj"

SCHEME=${LEGACYSTORE_CLIENT_SCHEME:-LegacyStore}
CONFIGURATION=${LEGACYSTORE_CLIENT_CONFIGURATION:-Release}
SDK=${LEGACYSTORE_CLIENT_SDK:-}
ARCHS=${LEGACYSTORE_CLIENT_ARCHS:-"i386 x86_64"}
DEPLOYMENT_TARGET=${LEGACYSTORE_CLIENT_DEPLOYMENT_TARGET:-10.5}
DERIVED_DATA=${LEGACYSTORE_CLIENT_DERIVED_DATA:-"$ROOT_DIR/build/DerivedData"}

if ! command -v xcodebuild >/dev/null 2>&1; then
  echo "xcodebuild is required to build the legacy Objective-C client." >&2
  exit 127
fi

MACOS_VERSION=$(sw_vers -productVersion 2>/dev/null || echo "unknown")
echo "Host macOS: $MACOS_VERSION"
echo "Client scheme: $SCHEME"
echo "Configuration: $CONFIGURATION"
echo "Architectures: $ARCHS"
echo "Deployment target: $DEPLOYMENT_TARGET"

case " $ARCHS " in
  *" i386 "*)
    ;;
  *)
    echo "LEGACYSTORE_CLIENT_ARCHS must include i386; refusing to silently drop it." >&2
    exit 1
    ;;
esac

if [ "$DEPLOYMENT_TARGET" != "10.5" ]; then
  echo "LEGACYSTORE_CLIENT_DEPLOYMENT_TARGET must remain 10.5 unless explicitly approved." >&2
  exit 1
fi

if [ ! -d "$PROJECT_PATH" ]; then
  echo "Objective-C client Xcode project is not implemented yet; this belongs to task 7." >&2
  exit 1
fi

set -- xcodebuild \
  -project "$PROJECT_PATH" \
  -scheme "$SCHEME" \
  -configuration "$CONFIGURATION" \
  -derivedDataPath "$DERIVED_DATA" \
  ARCHS="$ARCHS" \
  MACOSX_DEPLOYMENT_TARGET="$DEPLOYMENT_TARGET"

if [ -n "$SDK" ]; then
  set -- "$@" -sdk "$SDK"
fi

"$@"
