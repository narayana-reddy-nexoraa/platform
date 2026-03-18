.PHONY: docker-up docker-down migrate-up migrate-down sqlc-generate \
       build-api build-worker build-temporal-worker run-api run-worker \
       test test-unit test-integration test-e2e test-compliance test-load test-load-sop \
       docker-build docker-up-full docker-down-full \
       deploy tf-plan tf-apply tf-destroy \
       kafka-topics

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

build-temporal-worker:
	go build -o bin/temporal-worker ./cmd/temporal-worker

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

test-e2e:
	go test ./tests/e2e/... -v -count=1 -timeout 300s

test-compliance:
	go test ./internal/compliance/... -v -count=1

test-load:
	k6 run tests/k6/load_test.js

test-load-sop:
	k6 run tests/load/sop_load_test.js

# --- Docker Full Stack ---
docker-build:
	docker compose build

docker-up-full:
	docker compose up -d

docker-down-full:
	docker compose down -v

# --- AWS Deployment ---
deploy:
	@bash scripts/deploy.sh

tf-plan:
	cd terraform && terraform plan

tf-apply:
	cd terraform && terraform apply

tf-destroy:
	cd terraform && terraform destroy

# --- Kafka ---
kafka-topics:
	docker compose exec kafka kafka-topics --bootstrap-server localhost:29092 --list
