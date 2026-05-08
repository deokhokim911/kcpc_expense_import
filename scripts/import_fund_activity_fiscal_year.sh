#!/usr/bin/env bash
# 해당 회계연도(fiscalyear)의 fund_activity_import_run 전부 삭제 → CASCADE로 summary/detail 제거 후
# Summary + General Ledger(Special)를 한 번에 적재 (org_* 미변경)
#
# 사용:
#   ./scripts/import_fund_activity_fiscal_year.sh              # DB 적재
#   ./scripts/import_fund_activity_fiscal_year.sh --dry-run    # 삭제 대상 건수·파싱 결과만 확인
#
# 환경변수:
#   FISCAL_YEAR              회계연도 (기본 2026)
#   FUND_SUMMARY_XLSX        Fund Activity Summary 파일
#   FUND_DETAIL_LEDGER_XLSX  General Ledger Special web posting 파일

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FY="${FISCAL_YEAR:-2026}"
SUMMARY="${FUND_SUMMARY_XLSX:-$ROOT/2026/FundActivitySummary.20260503.xlsx}"
LEDGER="${FUND_DETAIL_LEDGER_XLSX:-$ROOT/2026/GeneralLedger.20260501.Special_web_posting.xlsx}"

MODE="live"
DRY=()
if [[ "${1:-}" == "--dry-run" ]] || [[ "${1:-}" == "-n" ]]; then
  DRY=( -dry-run )
  MODE="dry-run"
fi

if [[ ! -f "$SUMMARY" ]]; then
  echo "파일 없음: $SUMMARY" >&2
  exit 1
fi
if [[ ! -f "$LEDGER" ]]; then
  echo "파일 없음: $LEDGER" >&2
  exit 1
fi

echo "==> Fund Activity 전체 재적재 (같은 fiscalyear 기존 run·summary·detail 삭제 후 1건으로 재삽입)" >&2
echo "    fiscal_year=$FY mode=$MODE" >&2
echo "    summary: $SUMMARY" >&2
echo "    ledger:  $LEDGER" >&2

exec go run . \
  -replace-fund-activity-year \
  -fiscal-year "$FY" \
  -summary-xlsx "$SUMMARY" \
  -detail-ledger-xlsx "$LEDGER" \
  "${DRY[@]+"${DRY[@]}"}"
