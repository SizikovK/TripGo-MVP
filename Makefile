-include .env
export

.PHONY: generate migrate migrate-up migrate-down migrate-status run test test-integration lint build psql docker-build docker-run

build:
	go build ./...

run:
	go run ./cmd/trip-service

test:
	go test -race -count=2 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-integration:
	go test -race -tags integration -count=2 ./...

lint:
	go tool golangci-lint run

migrate: migrate-up

generate:
	go tool oapi-codegen -config api/oapi-codegen.yaml contracts/openapi/trip-service.openapi.yaml
	go generate ./internal/usecase/... ./internal/http/trip/...

migrate-up:
	go tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	go tool goose -dir migrations postgres "$$DATABASE_URL" down

migrate-status:
	go tool goose -dir migrations postgres "$$DATABASE_URL" status

psql:
	psql "$$DATABASE_URL"

docker-build:
	docker build -t tripgo .

docker-run: docker-build
	docker run --rm --env-file .env -e HTTP_ADDR=:8080 -e DATABASE_URL="$$DOCKER_DATABASE_URL" -p 8001:8080 tripgo
