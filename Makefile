APP_NAME := music-stream

.PHONY: run-api run-worker stage-media import-track migrate-up migrate-down docker-migrate-up docker-migrate-down docker-infra-up fmt test test-integration docker-up docker-down

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

stage-media:
	go run ./cmd/stage-media $(ARGS)

import-track:
	go run ./cmd/import-track $(ARGS)

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

docker-migrate-up:
	docker compose -f deploy/docker-compose.yml run --rm migrate up

docker-migrate-down:
	docker compose -f deploy/docker-compose.yml run --rm migrate down

docker-infra-up:
	docker compose -f deploy/docker-compose.yml up -d postgres redis minio

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

test-integration:
	go test -tags=integration ./e2e -count=1

docker-up:
	docker compose -f deploy/docker-compose.yml up -d --build

docker-down:
	docker compose -f deploy/docker-compose.yml down
