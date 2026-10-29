-include .env
export

DOCKER_IMAGE ?= tripgo
DOCKER_PORT ?= 8001

.PHONY: generate migrate migrate-up migrate-down migrate-status run test test-integration build psql docker-build docker-run

build:
	go build ./...

run:
	go run ./cmd/trip-service

test:
	go test -race ./...

test-integration:
	TRIPGO_INTEGRATION=1 go test -race ./... -count=1 -v

migrate: migrate-up

generate:
	go tool oapi-codegen -config api/oapi-codegen.yaml contracts/openapi/trip-service.openapi.yaml

migrate-up:
	go tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	go tool goose -dir migrations postgres "$$DATABASE_URL" down

migrate-status:
	go tool goose -dir migrations postgres "$$DATABASE_URL" status

psql:
	psql "$$DATABASE_URL"

# Docker Desktop: локальный адрес БД заменяется только для контейнера.
docker-build:
	docker build -t "$(DOCKER_IMAGE)" .

docker-run: docker-build
	@set -eu; \
	container_database_url="$${DOCKER_DATABASE_URL:-}"; \
	if [ -z "$$container_database_url" ]; then \
		container_database_url=$$(printf '%s' "$$DATABASE_URL" | sed -e 's|@localhost:|@host.docker.internal:|' -e 's|@127\.0\.0\.1:|@host.docker.internal:|'); \
	fi; \
	if [ -z "$$container_database_url" ]; then \
		echo 'DATABASE_URL обязателен: сначала запустите tripgoctl environment start' >&2; \
		exit 1; \
	fi; \
	docker run --rm --env-file .env \
		-e HTTP_ADDR=:8080 \
		-e DATABASE_URL="$$container_database_url" \
		-p "$(DOCKER_PORT):8080" "$(DOCKER_IMAGE)"
