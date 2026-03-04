#!/usr/bin/env bash

API_URL="${API_URL:-http://localhost:8080}"
TENANT_ID="a0000000-0000-0000-0000-000000000001"
COMPOSE_FILES="-f docker-compose.yml -f docker-compose.demo.yml"

create_execution() {
  local key
  key=$(uuidgen 2>/dev/null || python3 -c 'import uuid; print(uuid.uuid4())')
  curl -sf -X POST "$API_URL/api/v1/executions" \
    -H "Content-Type: application/json" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Idempotency-Key: $key" \
    -d '{"payload":{"task":"demo","key":"'"$key"'"}}' 2>/dev/null
}

count_by_status() {
  local status=$1
  docker compose $COMPOSE_FILES exec -T postgres \
    psql -U narayana -d narayana -t -A \
    -c "SELECT COUNT(*) FROM executions WHERE status = '$status';"
}

count_total() {
  docker compose $COMPOSE_FILES exec -T postgres \
    psql -U narayana -d narayana -t -A \
    -c "SELECT COUNT(*) FROM executions;"
}

truncate_executions() {
  docker compose $COMPOSE_FILES exec -T postgres \
    psql -U narayana -d narayana \
    -c "TRUNCATE executions, execution_transitions, outbox_events, processed_events, dead_letter_events, processing_log, consumer_offsets CASCADE;"
}

wait_for_status() {
  local target_status=$1
  local expected_count=$2
  local timeout=${3:-120}
  local elapsed=0

  echo "Waiting for $expected_count executions in $target_status (timeout: ${timeout}s)..."
  while [ $elapsed -lt $timeout ]; do
    local count
    count=$(count_by_status "$target_status")
    if [ "$count" -ge "$expected_count" ]; then
      echo ""
      echo "✓ $count executions reached $target_status"
      return 0
    fi
    printf "\r  Progress: %s / %s in %s (%ss elapsed)" "$count" "$expected_count" "$target_status" "$elapsed"
    sleep 2
    elapsed=$((elapsed + 2))
  done
  echo ""
  echo "✗ Timeout! Only $(count_by_status "$target_status") reached $target_status"
  return 1
}
