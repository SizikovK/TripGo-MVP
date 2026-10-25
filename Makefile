-include .env
export

.PHONY: generate migrate migrate-up migrate-down migrate-status run test test-integration build psql

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
