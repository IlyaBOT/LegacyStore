# LegacyStore development notes

## Current iteration

Implemented:

- repository scaffold;
- Docker Compose with PostgreSQL, backend and static admin-web service;
- Go backend public catalog API;
- `GET /api/v1/bootstrap`;
- `GET /api/v1/categories`;
- `GET /api/v1/apps`;
- `GET /api/v1/apps/{slug}`;
- `GET /api/v1/apps/{slug}/versions`;
- `GET /api/v1/apps/{slug}/reviews`;
- `GET /api/v1/search`;
- `GET /api/v1/download/{artifact_id}`;
- PostgreSQL migrations;
- compatibility engine;
- seed data;
- curl/jq API tests;
- account auth, roles, reviews, legacy passwords, 2FA and moderation APIs;
- static web client with nginx API proxy.

Not implemented yet:

- Objective-C client skeleton;
- uploads, OAuth, CDN and P2P workflows.
