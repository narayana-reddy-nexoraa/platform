#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/helpers.sh"

echo "============================================"
echo "  SCENARIO 2: Crash Recovery"
echo "============================================"
echo ""

truncate_executions > /dev/null

echo "=== Creating 200 executions ==="
for i in $(seq 1 200); do
  create_execution > /dev/null &
  if (( i % 50 == 0 )); then
    wait
    echo "  Batch $((i/50))/4 sent"
  fi
done
wait
echo "✓ All 200 creation requests sent"
echo ""

sleep 3
echo "=== Current state before crash ==="
echo "  Running:  $(count_by_status 'RUNNING')"
echo "  Claimed:  $(count_by_status 'CLAIMED')"
echo "  Created:  $(count_by_status 'CREATED')"
echo ""

echo "=== KILLING worker-3 and worker-4 ==="
docker compose $COMPOSE_FILES stop worker-3 worker-4
echo "✓ Workers 3 and 4 stopped"
echo ""

echo "=== Waiting for leases to expire and reaper to reclaim (~35s) ==="
echo "  (Watch Grafana: leases_reclaimed_total should spike)"
sleep 35

echo ""
echo "=== State after reaper runs ==="
echo "  Succeeded: $(count_by_status 'SUCCEEDED')"
echo "  Running:   $(count_by_status 'RUNNING')"
echo "  Created:   $(count_by_status 'CREATED') (reclaimed, waiting to be re-claimed)"
echo ""

echo "=== Waiting for remaining workers to finish all 200 ==="
wait_for_status "SUCCEEDED" 200 180

echo ""
echo "=== Restarting killed workers ==="
docker compose $COMPOSE_FILES start worker-3 worker-4

echo ""
echo "=== Verification ==="
echo "Total:       $(count_total)"
echo "Succeeded:   $(count_by_status 'SUCCEEDED')"

RECLAIMS=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT COUNT(*) FROM execution_transitions WHERE from_status = 'RUNNING' AND to_status = 'CREATED';")
echo "Reclaim transitions (RUNNING → CREATED): $RECLAIMS"

echo ""
if [ "$(count_by_status 'SUCCEEDED')" -eq 200 ]; then
  echo "✓ SCENARIO 2 PASSED: All 200 completed despite 2 worker crashes"
else
  echo "✗ SCENARIO 2 NEEDS INVESTIGATION"
fi
