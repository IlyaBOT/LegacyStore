# Current Task

Implement the initial LegacyStore repository scaffold.

## Required files

Create or update:

- README.md
- AGENTS.md
- docker-compose.yml
- .env.example
- Makefile
- scripts/build_backend.sh
- scripts/build_web.sh
- scripts/build_client.sh
- scripts/dev_up.sh
- scripts/dev_down.sh
- scripts/migrate.sh
- scripts/seed.sh
- scripts/test_api.sh
- backend/
- admin-web/
- legacy-client/
- docs/

## First implementation scope

Do not implement the full product yet.

Implement only:

1. Go backend skeleton.
2. PostgreSQL Docker Compose service.
3. Basic REST API endpoints:
   - GET /api/v1/bootstrap
   - GET /api/v1/categories
   - GET /api/v1/apps
   - GET /api/v1/apps/{slug}
   - GET /api/v1/search
4. PostgreSQL migrations.
5. Fixture seed data.
6. API test script using curl and jq.
7. Russian README.md.
8. Placeholder Objective-C client project structure.
9. Placeholder admin web UI structure.

## Do not implement yet

- Real P2P downloader.
- Real OAuth.
- Real file uploads.
- Real payment system.
- Real CDN.
- Full moderation panel.
- Full native client UI.