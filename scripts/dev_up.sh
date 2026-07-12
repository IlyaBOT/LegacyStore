#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

CERT_DIR="$ROOT_DIR/admin-web/certs"
CERT_FILE="$CERT_DIR/localhost.crt"
KEY_FILE="$CERT_DIR/localhost.key"

if [ -f .env ]; then
  set -a
  . ./.env
  set +a
else
  echo "No .env file found; Docker Compose will use development defaults."
fi

if [ ! -f "$CERT_FILE" ] || [ ! -f "$KEY_FILE" ]; then
  mkdir -p "$CERT_DIR"
  openssl req \
    -x509 \
    -newkey rsa:2048 \
    -nodes \
    -keyout "$KEY_FILE" \
    -out "$CERT_FILE" \
    -days 3650 \
    -subj "/CN=localhost" \
    -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
fi

docker compose up -d --build

BACKEND_PORT=${BACKEND_PORT:-8080}
ADMIN_WEB_PORT=${ADMIN_WEB_PORT:-8081}
ADMIN_WEB_HTTPS_PORT=${ADMIN_WEB_HTTPS_PORT:-8443}

echo "Backend: http://localhost:$BACKEND_PORT"
echo "Bootstrap: http://localhost:$BACKEND_PORT/api/v1/bootstrap"
echo "Admin web: http://localhost:$ADMIN_WEB_PORT"
echo "Admin web HTTPS: https://localhost:$ADMIN_WEB_HTTPS_PORT"
