#!/usr/bin/env bash
# Step-load benchmark via docker compose. Uses --no-build so no Dockerfile/build runs
# on the host (ledger-engine must already exist locally, e.g. pulled from ECR).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

mkdir -p benchmarks

STEPS=(200 400 600 800 1000 1200)
DURATION="${BENCH_DURATION:-120s}"
CONCURRENCY="${BENCH_CONCURRENCY:-20}"
BATCH_SIZE="${BENCH_BATCH_SIZE:-5}"
ACCOUNTS="${BENCH_ACCOUNTS:-100}"

SUMMARY_FILE="benchmarks/ramp_summary_$(date +%Y%m%d_%H%M%S).csv"
echo "rps_target,duration,requests_total,success_total,failed_total,avg_latency_ms,p95_latency_ms,p99_latency_ms,batch_size,result_file" > "${SUMMARY_FILE}"

echo "Running ramp benchmark..."
for RPS in "${STEPS[@]}"; do
  OUT_FILE="benchmarks/trafficgen_rps_${RPS}.csv"
  echo "Step target RPS=${RPS}"

  docker compose run --rm --no-build \
    -e TRAFFICGEN_BASE_URL=http://ledger-engine:8080 \
    -e TRAFFICGEN_API_KEY=dev-key-12345 \
    -e TRAFFICGEN_DURATION="${DURATION}" \
    -e TRAFFICGEN_CONCURRENCY="${CONCURRENCY}" \
    -e TRAFFICGEN_RPS="${RPS}" \
    -e TRAFFICGEN_BATCH_SIZE="${BATCH_SIZE}" \
    -e TRAFFICGEN_ACCOUNTS="${ACCOUNTS}" \
    -e TRAFFICGEN_CURRENCY=USD \
    -e TRAFFICGEN_MIN_AMOUNT=1 \
    -e TRAFFICGEN_MAX_AMOUNT=100 \
    -e TRAFFICGEN_CSV_OUTPUT="/app/${OUT_FILE}" \
    trafficgen >/dev/null

  REQUESTS="$(awk -F, '$1=="requests_total"{print $2}' "${OUT_FILE}")"
  SUCCESS="$(awk -F, '$1=="success_total"{print $2}' "${OUT_FILE}")"
  FAILED="$(awk -F, '$1=="failed_total"{print $2}' "${OUT_FILE}")"
  AVG="$(awk -F, '$1=="avg_latency_ms"{print $2}' "${OUT_FILE}")"
  P95="$(awk -F, '$1=="p95_latency_ms"{print $2}' "${OUT_FILE}")"
  P99="$(awk -F, '$1=="p99_latency_ms"{print $2}' "${OUT_FILE}")"
  BATCH="$(awk -F, '$1=="batch_size"{print $2}' "${OUT_FILE}")"

  echo "${RPS},${DURATION},${REQUESTS},${SUCCESS},${FAILED},${AVG},${P95},${P99},${BATCH},${OUT_FILE}" >> "${SUMMARY_FILE}"

  # Stop ramp when reliability drops beyond 1% failures.
  if [[ "${REQUESTS}" -gt 0 ]]; then
    FAIL_PCT="$(awk -v f="${FAILED}" -v r="${REQUESTS}" 'BEGIN { printf "%.4f", (f/r)*100 }')"
    echo "Failure rate: ${FAIL_PCT}%"
    awk -v fp="${FAIL_PCT}" 'BEGIN { exit !(fp > 1.0) }' || continue
    echo "Stopping ramp due to failure rate > 1%."
    break
  fi
done

echo "Ramp completed. Summary: ${SUMMARY_FILE}"
