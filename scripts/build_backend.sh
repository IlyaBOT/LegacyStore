#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BUILD_DIR="$ROOT_DIR/build"

cd "$ROOT_DIR"

mkdir -p "$BUILD_DIR"

docker compose build backend
docker compose run --rm --no-deps --entrypoint sh backend -c '
  set -eu
  cd /app
  GO_FILES=$(find . -name "*.go" -not -path "./vendor/*")
  if [ -n "$GO_FILES" ]; then
    gofmt -w $GO_FILES
  fi
  go test ./...
  go build -o /workspace/build/legacystore-backend ./cmd/legacystore-backend
'

echo "Backend built: $BUILD_DIR/legacystore-backend"
