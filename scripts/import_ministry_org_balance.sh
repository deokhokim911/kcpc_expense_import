#!/usr/bin/env bash
# 사역원 일반 General Ledger → public.org_balance 적재 (첫 번째 시트, handleMinistryBalance)
# 특별(Special) 원장은 사용하지 않음 — Fund Activity 스크립트 참조.
#
# 사용:
#   ./scripts/import_ministry_org_balance.sh              # DB 적재 (실행 시 해당 회계연도 org_balance 삭제 후 재삽입 — main.go 의 fiscalYear 기준)
#   ./scripts/import_ministry_org_balance.sh --dry-run    # 파싱만 (DB INSERT 없음)
#
# 경로·옵션:
#   export MINISTRY_LEDGER_XLSX=/path/to/GeneralLedger_web_posting.xlsx
#   export MINISTRY_IMPORT_QUIET=1   # 행 단위 출력 숨김 (-quiet)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

# shellcheck source=lib/load_run_all_imports_env.sh
source "$SCRIPT_DIR/lib/load_run_all_imports_env.sh"
kcpc_resolve_import_env_paths
kcpc_check_import_db_env

LEDGER="${MINISTRY_LEDGER_XLSX:-$ROOT/2026/GeneralLedger.20260501_web_posting.xlsx}"

DRY_RUN=()
if [[ "${1:-}" == "--dry-run" ]] || [[ "${1:-}" == "-n" ]]; then
  DRY_RUN=( -dry-run )
fi

QUIET_FLAG=()
if [[ "${MINISTRY_IMPORT_QUIET:-}" == "1" ]]; then
  QUIET_FLAG=( -quiet )
fi

if [[ ! -f "$LEDGER" ]]; then
  echo "파일이 없습니다: $LEDGER" >&2
  echo "MINISTRY_LEDGER_XLSX 로 지정하거나 2026 폴더에 사역원 원장 엑셀을 두세요." >&2
  exit 1
fi

MODE="live"
if [[ ${#DRY_RUN[@]} -gt 0 ]]; then
  MODE="dry-run"
fi

echo "==> Ministry org_balance import (일반 웹 포스팅 원장 → handleMinistryBalance)" >&2
echo "    ledger: $LEDGER" >&2
echo "    mode=$MODE (MINISTRY_IMPORT_QUIET=${MINISTRY_IMPORT_QUIET:-0})" >&2
echo "    (go run 컴파일·DB 작업은 시작 후 로그가 출력됩니다)" >&2

exec go run . \
  -import-org-balance \
  -ministry-ledger-xlsx "$LEDGER" \
  "${QUIET_FLAG[@]+"${QUIET_FLAG[@]}"}" \
  "${DRY_RUN[@]+"${DRY_RUN[@]}"}"
