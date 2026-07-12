#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

cd "$ROOT_DIR"

if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi

if ! docker compose ps --status running postgres >/dev/null 2>&1; then
  echo "PostgreSQL container is not running. Start it with: make dev-up" >&2
  exit 1
fi

echo "Waiting for PostgreSQL..."
until docker compose run --rm --entrypoint sh backend -c 'pg_isready -d "$DATABASE_URL"' >/dev/null 2>&1; do
  sleep 1
done

docker compose run --rm --entrypoint sh backend -c \
  'psql -v ON_ERROR_STOP=1 "$DATABASE_URL" < /app/fixtures/seed.sql'

echo "Seed data loaded."
