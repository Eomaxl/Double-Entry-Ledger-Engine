#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

if [[ ! -f ".env" ]]; then
  echo "Missing .env file. Create it from .env.production.example first."
  exit 1
fi

echo "Pulling latest images/build context..."
docker compose pull || true

echo "Starting core services..."
docker compose up -d --build postgres nats ledger-engine

echo "Waiting briefly before health checks..."
sleep 8

echo "Checking health endpoints..."
curl -fsS "http://localhost:8080/health" >/dev/null
curl -fsS "http://localhost:8080/health/live" >/dev/null
curl -fsS "http://localhost:8080/health/ready" >/dev/null

echo "Deployment successful."
