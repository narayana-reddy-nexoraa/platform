#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/helpers.sh"

echo "============================================"
echo "  FULL VERIFICATION"
echo "============================================"
echo ""

echo "=== Execution Status Summary ==="
docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana \
  -c "SELECT status, COUNT(*) FROM executions GROUP BY status ORDER BY status;"
echo ""

echo "=== Stuck Executions (non-terminal for >5 minutes) ==="
STUCK=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT COUNT(*) FROM executions WHERE status NOT IN ('SUCCEEDED', 'CANCELED') AND updated_at < NOW() - INTERVAL '5 minutes';")
echo "Stuck executions: $STUCK"

echo ""
echo "=== Duplicate Processing Check ==="
DUPS=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT COUNT(*) FROM (SELECT execution_id FROM processing_log WHERE action = 'COMPLETED' GROUP BY execution_id HAVING COUNT(*) > 1) t;")
echo "Duplicate completions: $DUPS"

echo ""
echo "=== Invalid Transitions ==="
INVALID=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT COUNT(*) FROM execution_transitions WHERE (from_status, to_status) NOT IN (('CREATED','CLAIMED'),('CREATED','CANCELED'),('CLAIMED','RUNNING'),('CLAIMED','CREATED'),('CLAIMED','CANCELED'),('CLAIMED','TIMED_OUT'),('RUNNING','SUCCEEDED'),('RUNNING','FAILED'),('RUNNING','CANCELED'),('RUNNING','TIMED_OUT'),('FAILED','CREATED'),('FAILED','CLAIMED'),('FAILED','CANCELED'),('TIMED_OUT','CREATED'));")
echo "Invalid transitions: $INVALID"

echo ""
echo "=== Outbox Health ==="
docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana \
  -c "SELECT sent, COUNT(*) FROM outbox_events GROUP BY sent;"

echo ""
echo "=== DLQ ==="
DLQ=$(docker compose $COMPOSE_FILES exec -T postgres \
  psql -U narayana -d narayana -t -A \
  -c "SELECT COUNT(*) FROM dead_letter_events;")
echo "Dead letter events: $DLQ"

echo ""
echo "============================================"
if [ "$STUCK" -eq 0 ] && [ "$DUPS" -eq 0 ] && [ "$INVALID" -eq 0 ]; then
  echo "✓ ALL VERIFICATION CHECKS PASSED"
else
  echo "✗ SOME CHECKS FAILED — investigate above"
fi
echo "============================================"
