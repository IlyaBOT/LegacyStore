#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
WEB_OUT="$ROOT_DIR/build/admin-web"

mkdir -p "$WEB_OUT"
cp "$ROOT_DIR/admin-web/nginx.conf" "$WEB_OUT"/nginx.conf

echo "LegacyStore browser UI is embedded into the Go backend; no separate frontend bundle is built."
echo "nginx reverse-proxy config prepared: $WEB_OUT/nginx.conf"
