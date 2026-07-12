.PHONY: dev-up dev-down backend web client migrate seed test-api test-backend clean

dev-up:
	./scripts/dev_up.sh

dev-down:
	./scripts/dev_down.sh

backend:
	./scripts/build_backend.sh

web:
	./scripts/build_web.sh

client:
	./scripts/build_client.sh

migrate:
	./scripts/migrate.sh

seed:
	./scripts/seed.sh

test-api:
	./scripts/test_api.sh

test-backend:
	docker compose build backend
	docker compose run --rm --no-deps --entrypoint sh backend -c 'cd /app && go test ./...'

clean:
	rm -rf build
