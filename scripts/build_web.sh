#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
WEB_SRC="$ROOT_DIR/admin-web/static"
WEB_OUT="$ROOT_DIR/build/admin-web"

mkdir -p "$WEB_OUT"
cp -R "$WEB_SRC"/. "$WEB_OUT"/
cp "$ROOT_DIR/admin-web/nginx.conf" "$WEB_OUT"/nginx.conf

echo "Admin web uses static assets; no Node.js frontend build step is required."
echo "Web assets prepared: $WEB_OUT"
