#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/helpers.sh"

echo "============================================"
echo "  SCENARIO 3: Retry Storm Handling"
echo "============================================"
echo ""

truncate_executions > /dev/null

echo "=== Enabling 50% failure injection on all workers ==="
docker compose $COMPOSE_FILES stop worker-1 worker-2 worker-3 worker-4
FAILURE_RATE=0.5 docker compose $COMPOSE_FILES up -d worker-1 worker-2 worker-3 worker-4
echo "✓ Workers restarted with FAILURE_RATE=0.5"
sleep 5
echo ""

echo "=== Creating 100 executions ==="
for i in $(seq 1 100); do
  create_execution > /dev/null &
  if (( i % 50 == 0 )); then
    wait
  fi
done
wait
echo "✓ All 100 creation requests sent"
echo ""

echo "=== Watching retry behavior (30s with failures active) ==="
echo "  (Watch Grafana: executions_retried_total climbing)"
for i in $(seq 1 6); do
  sleep 5
  echo "  [${i}0s] Succeeded: $(count_by_status 'SUCCEEDED'), Failed: $(count_by_status 'FAILED'), Created: $(count_by_status 'CREATED')"
done
echo ""

echo "=== Disabling failure injection ==="
docker compose $COMPOSE_FILES stop worker-1 worker-2 worker-3 worker-4
FAILURE_RATE=0 docker compose $COMPOSE_FILES up -d worker-1 worker-2 worker-3 worker-4
echo "✓ Workers restarted with FAILURE_RATE=0"
sleep 5
echo ""

echo "=== Waiting for all executions to succeed ==="
wait_for_status "SUCCEEDED" 100 180

echo ""
echo "=== Verification ==="
echo "Total:      $(count_total)"
echo "Succeeded:  $(count_by_status 'SUCCEEDED')"

RETRY_COUNT=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT SUM(attempt_count) FROM executions WHERE status = 'SUCCEEDED';")
echo "Total attempts across all executions: $RETRY_COUNT"

echo ""
if [ "$(count_by_status 'SUCCEEDED')" -eq 100 ]; then
  echo "✓ SCENARIO 3 PASSED: All 100 eventually succeeded after retries"
else
  echo "✗ SCENARIO 3 NEEDS INVESTIGATION"
fi
