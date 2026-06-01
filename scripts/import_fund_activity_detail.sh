#!/usr/bin/env bash
# General Ledger(Special web posting) → fund_activity_detail_line 적재 (org_* 미사용)
# 동일 보고 기간의 fund_activity_import_run 이 이미 있어야 함 (Summary 적재 후 또는 기존 run).
#
# 사용:
#   ./scripts/import_fund_activity_detail.sh                     # DB 적재, run은 기간으로 자동 매칭
#   RUN_ID=2 ./scripts/import_fund_activity_detail.sh            # 특정 run_id
#   ./scripts/import_fund_activity_detail.sh --dry-run
#
# 환경변수:
#   FUND_DETAIL_LEDGER_XLSX  General Ledger 파일 경로 (기본: 2026/GeneralLedger.20260501.Special_web_posting.xlsx)
#   RUN_ID                   숫자면 -run-id 로 전달 (-match-run-by-period 생략)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

# shellcheck source=lib/load_run_all_imports_env.sh
source "$SCRIPT_DIR/lib/load_run_all_imports_env.sh"
kcpc_resolve_import_env_paths
kcpc_check_import_db_env

LEDGER="${FUND_DETAIL_LEDGER_XLSX:-$ROOT/2026/GeneralLedger.20260501.Special_web_posting.xlsx}"

DRY=()
if [[ "${1:-}" == "--dry-run" ]] || [[ "${1:-}" == "-n" ]]; then
  DRY=( -dry-run )
fi

if [[ ! -f "$LEDGER" ]]; then
  echo "파일이 없습니다: $LEDGER" >&2
  exit 1
fi

echo "==> Fund Activity Detail import (fund_activity_detail_line, org_* untouched)" >&2
echo "    ledger: $LEDGER" >&2
if [[ -n "${RUN_ID:-}" ]] && [[ "$RUN_ID" =~ ^[0-9]+$ ]]; then
  echo "    run_id=$RUN_ID" >&2
else
  echo "    run_id=(match by period from ledger vs fund_activity_import_run)" >&2
fi
DMODE="live"
if [[ ${#DRY[@]} -gt 0 ]]; then
  DMODE="dry-run"
fi
echo "    mode=$DMODE" >&2
echo "    (go run 컴파일·DB 작업은 시작 후 로그가 출력됩니다)" >&2

ARGS=(
  -import-fund-detail
  -detail-ledger-xlsx "$LEDGER"
  "${DRY[@]+"${DRY[@]}"}"
)

if [[ -n "${RUN_ID:-}" ]] && [[ "$RUN_ID" =~ ^[0-9]+$ ]]; then
  ARGS+=( -run-id "$RUN_ID" )
else
  ARGS+=( -match-run-by-period )
fi

exec go run . "${ARGS[@]}"
