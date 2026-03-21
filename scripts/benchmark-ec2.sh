#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

TS="$(date +%Y%m%d_%H%M%S)"
OUT_DIR="${OUT_DIR:-benchmarks/ec2_${TS}}"
mkdir -p "${OUT_DIR}"

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
API_KEY="${API_KEY:-dev-key-12345}"
RPS_STEPS="${RPS_STEPS:-10 20 30 40 50 60 80 100}"
DURATION="${DURATION:-180s}"
CONCURRENCY="${CONCURRENCY:-20}"
BATCH_SIZE="${BATCH_SIZE:-5}"
ACCOUNTS="${ACCOUNTS:-100}"
CURRENCY="${CURRENCY:-USD}"
MIN_AMOUNT="${MIN_AMOUNT:-1}"
MAX_AMOUNT="${MAX_AMOUNT:-100}"

SUMMARY_FILE="${OUT_DIR}/summary.csv"
echo "target_rps,duration_ms,requests_total,success_total,failed_total,effective_rps,effective_tps,p95_ms,p99_ms,fail_pct,csv_file,log_file" > "${SUMMARY_FILE}"

echo "Starting EC2 benchmark run..."
echo "Output directory: ${OUT_DIR}"
echo "Base URL: ${BASE_URL}"
echo "Steps: ${RPS_STEPS}"

for RPS in ${RPS_STEPS}; do
  CSV_FILE="${OUT_DIR}/trafficgen_rps_${RPS}.csv"
  LOG_FILE="${OUT_DIR}/trafficgen_rps_${RPS}.log"

  echo ""
  echo "==> Running step target RPS=${RPS}"

  go run ./cmd/trafficgen \
    -base-url "${BASE_URL}" \
    -api-key "${API_KEY}" \
    -duration "${DURATION}" \
    -concurrency "${CONCURRENCY}" \
    -rps "${RPS}" \
    -batch-size "${BATCH_SIZE}" \
    -accounts "${ACCOUNTS}" \
    -currency "${CURRENCY}" \
    -min-amount "${MIN_AMOUNT}" \
    -max-amount "${MAX_AMOUNT}" \
    -csv-output "${CSV_FILE}" | tee "${LOG_FILE}"

  DURATION_MS="$(awk -F, '$1=="duration_ms"{print $2}' "${CSV_FILE}")"
  REQUESTS_TOTAL="$(awk -F, '$1=="requests_total"{print $2}' "${CSV_FILE}")"
  SUCCESS_TOTAL="$(awk -F, '$1=="success_total"{print $2}' "${CSV_FILE}")"
  FAILED_TOTAL="$(awk -F, '$1=="failed_total"{print $2}' "${CSV_FILE}")"
  P95_MS="$(awk -F, '$1=="p95_latency_ms"{print $2}' "${CSV_FILE}")"
  P99_MS="$(awk -F, '$1=="p99_latency_ms"{print $2}' "${CSV_FILE}")"

  if [[ -z "${DURATION_MS}" || "${DURATION_MS}" == "0" ]]; then
    EFFECTIVE_RPS="0"
    EFFECTIVE_TPS="0"
    FAIL_PCT="0"
  else
    EFFECTIVE_RPS="$(awk -v s="${SUCCESS_TOTAL}" -v ms="${DURATION_MS}" 'BEGIN { printf "%.2f", s/(ms/1000) }')"
    EFFECTIVE_TPS="$(awk -v r="${EFFECTIVE_RPS}" -v b="${BATCH_SIZE}" 'BEGIN { printf "%.2f", r*b }')"
    FAIL_PCT="$(awk -v f="${FAILED_TOTAL}" -v r="${REQUESTS_TOTAL}" 'BEGIN { if (r==0) {printf "0.00"} else {printf "%.2f", (f/r)*100} }')"
  fi

  echo "${RPS},${DURATION_MS},${REQUESTS_TOTAL},${SUCCESS_TOTAL},${FAILED_TOTAL},${EFFECTIVE_RPS},${EFFECTIVE_TPS},${P95_MS},${P99_MS},${FAIL_PCT},${CSV_FILE},${LOG_FILE}" >> "${SUMMARY_FILE}"
done

echo ""
echo "Benchmark complete."
echo "Summary: ${SUMMARY_FILE}"
echo "Tip: cat ${SUMMARY_FILE}"
