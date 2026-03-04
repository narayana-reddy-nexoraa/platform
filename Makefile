.PHONY: docker-up docker-down migrate-up migrate-down sqlc-generate \
       build-api build-worker run-api run-worker \
       test test-unit test-integration test-load \
       docker-build docker-up-full docker-down-full

# --- Infrastructure ---
docker-up:
	docker compose up -d

docker-down:
	docker compose down

# --- Migrations ---
migrate-up:
	migrate -database "postgres://narayana:narayana@localhost:5432/narayana?sslmode=disable" -path db/migrations up

migrate-down:
	migrate -database "postgres://narayana:narayana@localhost:5432/narayana?sslmode=disable" -path db/migrations down

# --- Code Generation ---
sqlc-generate:
	sqlc generate

# --- Build ---
build-api:
	go build -o bin/api ./cmd/api

build-worker:
	go build -o bin/worker ./cmd/worker

# --- Run ---
run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

# --- Tests ---
test: test-unit test-integration

test-unit:
	go test ./internal/... -v -count=1

test-integration:
	go test ./tests/integration/... -v -count=1 -timeout 120s

test-load:
	k6 run tests/k6/load_test.js

# --- Docker Full Stack ---
docker-build:
	docker compose build

docker-up-full:
	docker compose up -d

docker-down-full:
	docker compose down -v
