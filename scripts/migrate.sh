#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
MIGRATIONS_DIR="$ROOT_DIR/backend/migrations"

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
  'psql -v ON_ERROR_STOP=1 "$DATABASE_URL" -c "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());"'

found=0
for file in "$MIGRATIONS_DIR"/*.up.sql; do
  if [ ! -f "$file" ]; then
    continue
  fi

  found=1
  version=$(basename "$file" .up.sql)
  applied=$(docker compose run --rm --entrypoint sh backend -c \
    "psql -Atq \"\$DATABASE_URL\" -c \"SELECT 1 FROM schema_migrations WHERE version = '$version';\"")

  if [ "$applied" = "1" ]; then
    echo "Skipping already applied migration: $version"
    continue
  fi

  echo "Applying migration: $version"
  migration_path="/app/migrations/$(basename "$file")"
  docker compose run --rm --entrypoint sh backend -c \
    "psql -v ON_ERROR_STOP=1 \"\$DATABASE_URL\" < '$migration_path'"

  docker compose run --rm --entrypoint sh backend -c \
    "psql -v ON_ERROR_STOP=1 \"\$DATABASE_URL\" -c \"INSERT INTO schema_migrations (version) VALUES ('$version');\""
done

if [ "$found" -eq 0 ]; then
  echo "No migrations found in $MIGRATIONS_DIR" >&2
  exit 1
fi

echo "Migrations finished."
