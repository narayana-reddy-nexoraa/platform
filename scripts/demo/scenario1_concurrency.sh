#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/helpers.sh"

echo "============================================"
echo "  SCENARIO 1: High Concurrency Safety"
echo "============================================"
echo ""

truncate_executions > /dev/null

echo "=== Creating 500 executions concurrently (50 parallel) ==="
for i in $(seq 1 500); do
  create_execution > /dev/null &
  if (( i % 50 == 0 )); then
    wait
    echo "  Batch $((i/50))/10 sent"
  fi
done
wait
echo "✓ All 500 creation requests sent"
echo ""

echo "=== Waiting for workers to process ==="
wait_for_status "SUCCEEDED" 500 180

echo ""
echo "=== Verification Queries ==="
echo "Total executions:   $(count_total)"
echo "Succeeded:          $(count_by_status 'SUCCEEDED')"
echo "Failed:             $(count_by_status 'FAILED')"
echo "Still running:      $(count_by_status 'RUNNING')"
echo "Still claimed:      $(count_by_status 'CLAIMED')"

DUPS=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT COUNT(*) FROM (SELECT execution_id FROM processing_log WHERE action = 'COMPLETED' GROUP BY execution_id HAVING COUNT(*) > 1) t;")
echo "Duplicate completions: $DUPS"

echo ""
if [ "$(count_by_status 'SUCCEEDED')" -eq 500 ] && [ "$DUPS" -eq 0 ]; then
  echo "✓ SCENARIO 1 PASSED: 500/500 succeeded, 0 duplicates"
else
  echo "✗ SCENARIO 1 NEEDS INVESTIGATION"
fi
