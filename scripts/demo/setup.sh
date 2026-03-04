#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILES="-f docker-compose.yml -f docker-compose.demo.yml"

echo "=== Building images ==="
docker compose $COMPOSE_FILES build

echo "=== Starting stack ==="
docker compose $COMPOSE_FILES up -d

echo "=== Waiting for API to be healthy ==="
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/health/ready > /dev/null 2>&1; then
    echo "API is ready!"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: API did not become healthy in 60s"
    exit 1
  fi
  echo "Waiting... ($i/30)"
  sleep 2
done

echo ""
echo "=== Running containers ==="
docker compose $COMPOSE_FILES ps

echo ""
echo "=== Setup complete ==="
echo "Grafana:    http://localhost:3001 (admin/admin)"
echo "Prometheus: http://localhost:9091"
echo "API:        http://localhost:8080"
