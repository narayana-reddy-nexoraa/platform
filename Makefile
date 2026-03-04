.PHONY: docker-up docker-down migrate-up migrate-down sqlc-generate \
       build-api build-worker run-api run-worker \
       test test-unit test-integration test-load \
       docker-build docker-up-full docker-down-full \
       demo-setup demo-teardown demo-scenario1 demo-scenario2 \
       demo-scenario3 demo-scenario4 demo-verify \
       deploy tf-plan tf-apply tf-destroy

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

# --- Demo ---
demo-setup:
	@bash scripts/demo/setup.sh

demo-teardown:
	@bash scripts/demo/teardown.sh

demo-scenario1:
	@bash scripts/demo/scenario1_concurrency.sh

demo-scenario2:
	@bash scripts/demo/scenario2_crash_recovery.sh

demo-scenario3:
	@bash scripts/demo/scenario3_retry_storm.sh

demo-scenario4:
	@bash scripts/demo/scenario4_event_pipeline.sh

demo-verify:
	@bash scripts/demo/verify.sh

# --- AWS Deployment ---
deploy:
	@bash scripts/deploy.sh

tf-plan:
	cd terraform && terraform plan

tf-apply:
	cd terraform && terraform apply

tf-destroy:
	cd terraform && terraform destroy
