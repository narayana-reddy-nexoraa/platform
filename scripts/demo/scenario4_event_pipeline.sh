#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/helpers.sh"

echo "============================================"
echo "  SCENARIO 4: Idempotent Event Processing"
echo "============================================"
echo ""

truncate_executions > /dev/null

echo "=== Creating 50 executions ==="
for i in $(seq 1 50); do
  create_execution > /dev/null &
done
wait
echo "✓ All 50 creation requests sent"
echo ""

echo "=== Waiting for all to complete ==="
wait_for_status "SUCCEEDED" 50 120

echo ""
echo "=== Event Pipeline Verification ==="

OUTBOX_TOTAL=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT COUNT(*) FROM outbox_events;")
OUTBOX_SENT=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT COUNT(*) FROM outbox_events WHERE sent = TRUE;")
OUTBOX_UNSENT=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT COUNT(*) FROM outbox_events WHERE sent = FALSE;")
echo "Outbox events total:  $OUTBOX_TOTAL"
echo "Outbox events sent:   $OUTBOX_SENT"
echo "Outbox events unsent: $OUTBOX_UNSENT"
echo ""

PROCESSED=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT COUNT(*) FROM processed_events;")
echo "Processed events (deduped): $PROCESSED"

DLQ_COUNT=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT COUNT(*) FROM dead_letter_events;")
echo "Dead letter queue entries:  $DLQ_COUNT"
echo ""

echo "=== Event Types ==="
docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana \
  -c "SELECT event_type, COUNT(*) FROM outbox_events GROUP BY event_type ORDER BY COUNT(*) DESC;"
echo ""

echo "=== Tracing Single Execution ==="
EXEC_ID=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT execution_id FROM executions LIMIT 1;")
echo "Execution: $EXEC_ID"

echo ""
echo "Transitions:"
docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana \
  -c "SELECT from_status, to_status, triggered_by, created_at FROM execution_transitions WHERE execution_id = '$EXEC_ID' ORDER BY created_at;"

echo ""
echo "Events:"
docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana \
  -c "SELECT event_type, sent, sent_at FROM outbox_events WHERE aggregate_id = '$EXEC_ID' ORDER BY sequence_number;"
echo ""

if [ "$DLQ_COUNT" -eq 0 ] && [ "$OUTBOX_UNSENT" -eq 0 ]; then
  echo "✓ SCENARIO 4 PASSED: All events published, processed, 0 DLQ, 0 unsent"
else
  echo "✗ SCENARIO 4 NEEDS INVESTIGATION"
fi
