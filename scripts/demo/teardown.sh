#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILES="-f docker-compose.yml -f docker-compose.demo.yml"

echo "=== Tearing down stack ==="
docker compose $COMPOSE_FILES down -v

echo "=== Teardown complete ==="
