#!/usr/bin/env bash
# Fund Activity Summary 엑셀 → fund_activity_import_run + fund_activity_summary_row 전체 적재
# (migration.sql 범위 내; org_* 테이블은 수정하지 않음)
#
# 사용:
#   ./scripts/import_fund_activity_summary.sh              # DB에 적재
#   ./scripts/import_fund_activity_summary.sh --dry-run  # 파싱만 검증
#
# 경로 변경(선택):
#   export FUND_SUMMARY_XLSX=/path/to/FundActivitySummary.xlsx
#   export FUND_DETAIL_XLSX=/path/to/GeneralLedger.Special_web_posting.xlsx
#   export FISCAL_YEAR=2026

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

# shellcheck source=lib/load_run_all_imports_env.sh
source "$SCRIPT_DIR/lib/load_run_all_imports_env.sh"
kcpc_resolve_import_env_paths
kcpc_check_import_db_env

SUMMARY_XLSX="${FUND_SUMMARY_XLSX:-$ROOT/2026/FundActivitySummary.20260503.xlsx}"
DETAIL_XLSX="${FUND_DETAIL_XLSX:-$ROOT/2026/GeneralLedger.20260501.Special_web_posting.xlsx}"
FY="${FISCAL_YEAR:-2026}"

DRY_RUN=()
if [[ "${1:-}" == "--dry-run" ]] || [[ "${1:-}" == "-n" ]]; then
  DRY_RUN=( -dry-run )
fi

for f in "$SUMMARY_XLSX"; do
  if [[ ! -f "$f" ]]; then
    echo "파일이 없습니다: $f" >&2
    echo "FUND_SUMMARY_XLSX 로 경로를 지정하거나 2026 폴더에 엑셀을 두세요." >&2
    exit 1
  fi
done

MODE="live"
if [[ ${#DRY_RUN[@]} -gt 0 ]]; then
  MODE="dry-run"
fi
echo "==> Fund Activity Summary import (org_* untouched)" >&2
echo "    summary: $SUMMARY_XLSX" >&2
if [[ -f "$DETAIL_XLSX" ]]; then
  echo "    detail (period check): $DETAIL_XLSX" >&2
fi
echo "    fiscal_year=$FY mode=$MODE" >&2
echo "    (go run 컴파일·DB 작업은 시작 후 로그가 출력됩니다)" >&2

if [[ ! -f "$DETAIL_XLSX" ]]; then
  echo "경고: Detail 엑셀이 없어 기간 교차검사·detail 파일명 없이 Summary만 적재합니다: $DETAIL_XLSX" >&2
  exec go run . \
    -import-fund-summary \
    -summary-xlsx "$SUMMARY_XLSX" \
    -fiscal-year "$FY" \
    "${DRY_RUN[@]+"${DRY_RUN[@]}"}"
fi

exec go run . \
  -import-fund-summary \
  -summary-xlsx "$SUMMARY_XLSX" \
  -detail-xlsx "$DETAIL_XLSX" \
  -fiscal-year "$FY" \
  "${DRY_RUN[@]+"${DRY_RUN[@]}"}"
