#!/usr/bin/env bash
# 사역원 org_balance + Fund Activity(해당 연도 Summary·Detail 전체 재적재) 를 순서대로 한 번에 실행.
#
# 포함:
#   1. import_ministry_org_balance.sh
#   2. import_fund_activity_fiscal_year.sh  (같은 fiscalyear의 fund_activity_* 전부 지운 뒤 Summary+G/L 한 트랜잭션 적재)
#
# 제외 (부분/대체 흐름 — fiscal_year 가 이미 Summary+Detail 을 처리함):
#   - import_fund_activity_summary.sh
#   - import_fund_activity_detail.sh
#
# 설정 우선순위:
#   1) 이 스크립트 실행 전에 이미 설정된 환경변수 ( FISCAL_YEAR=… ./run_all_imports.sh )
#   2) 선택 파일 run_all_imports.env (RUN_ALL_IMPORTS_ENV 로 경로 변경 가능)
# (run_all_imports.env 안에서 쓰는 이름)
#   FISCAL_YEAR, MINISTRY_LEDGER_XLSX, FUND_SUMMARY_XLSX, FUND_DETAIL_LEDGER_XLSX, MINISTRY_IMPORT_QUIET
#   ./scripts/run_all_imports.sh
#   FISCAL_YEAR=2026 MINISTRY_LEDGER_XLSX=/path/a.xlsx ./scripts/run_all_imports.sh
#   RUN_ALL_IMPORTS_ENV=/path/my.env ./scripts/run_all_imports.sh
#   ./scripts/run_all_imports.sh --dry-run

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

# 실행 전에 호출자가 넣은 값 (선택 env 파일이 덮어쓴 뒤 복구 — 우선 유지)
_RUN_ALL_CLI_SET_FISCAL_YEAR="${FISCAL_YEAR+x}"
_RUN_ALL_CLI_VAL_FISCAL_YEAR="${FISCAL_YEAR-}"
_RUN_ALL_CLI_SET_MINISTRY_LEDGER="${MINISTRY_LEDGER_XLSX+x}"
_RUN_ALL_CLI_VAL_MINISTRY_LEDGER="${MINISTRY_LEDGER_XLSX-}"
_RUN_ALL_CLI_SET_FUND_SUMMARY="${FUND_SUMMARY_XLSX+x}"
_RUN_ALL_CLI_VAL_FUND_SUMMARY="${FUND_SUMMARY_XLSX-}"
_RUN_ALL_CLI_SET_FUND_DETAIL_LEDGER="${FUND_DETAIL_LEDGER_XLSX+x}"
_RUN_ALL_CLI_VAL_FUND_DETAIL_LEDGER="${FUND_DETAIL_LEDGER_XLSX-}"
_RUN_ALL_CLI_SET_MINISTRY_QUIET="${MINISTRY_IMPORT_QUIET+x}"
_RUN_ALL_CLI_VAL_MINISTRY_QUIET="${MINISTRY_IMPORT_QUIET-}"

# ---------------------------------------------------------------------------
# [편집] 스크립트 안에서만 쓸 기본값 (위·env 에서 안 주어졌을 때만 최종 export 에 반영)
# 비워 두면("") 해당 하위 스크립트가 자체 경로를 사용합니다.
# 예: _RUN_ALL_DEFAULT_MINISTRY_LEDGER="${_RUN_ALL_DEFAULT_MINISTRY_LEDGER:-$ROOT/2026/GeneralLedger.20260501_web_posting.xlsx}"
# ---------------------------------------------------------------------------
_RUN_ALL_DEFAULT_FISCAL_YEAR="${_RUN_ALL_DEFAULT_FISCAL_YEAR:-2026}"
_RUN_ALL_DEFAULT_MINISTRY_LEDGER="${_RUN_ALL_DEFAULT_MINISTRY_LEDGER:-}"
_RUN_ALL_DEFAULT_FUND_SUMMARY="${_RUN_ALL_DEFAULT_FUND_SUMMARY:-}"
_RUN_ALL_DEFAULT_FUND_DETAIL_LEDGER="${_RUN_ALL_DEFAULT_FUND_DETAIL_LEDGER:-}"
_RUN_ALL_DEFAULT_MINISTRY_QUIET="${_RUN_ALL_DEFAULT_MINISTRY_QUIET:-0}"

# ---------------------------------------------------------------------------
# 선택 env 파일 (KEY=value, shell 형식)
# ---------------------------------------------------------------------------
_ENV_FILE="${RUN_ALL_IMPORTS_ENV:-$SCRIPT_DIR/run_all_imports.env}"
if [[ -f "$_ENV_FILE" ]]; then
  echo "run_all_imports: loading $_ENV_FILE" >&2
  set -a
  # shellcheck disable=SC1090
  source "$_ENV_FILE"
  set +a
fi

# 호출자가 스크립트 시작 전에 넣은 값은 env 파일보다 우선
if [[ "$_RUN_ALL_CLI_SET_FISCAL_YEAR" == x ]]; then export FISCAL_YEAR="$_RUN_ALL_CLI_VAL_FISCAL_YEAR"; fi
if [[ "$_RUN_ALL_CLI_SET_MINISTRY_LEDGER" == x ]]; then export MINISTRY_LEDGER_XLSX="$_RUN_ALL_CLI_VAL_MINISTRY_LEDGER"; fi
if [[ "$_RUN_ALL_CLI_SET_FUND_SUMMARY" == x ]]; then export FUND_SUMMARY_XLSX="$_RUN_ALL_CLI_VAL_FUND_SUMMARY"; fi
if [[ "$_RUN_ALL_CLI_SET_FUND_DETAIL_LEDGER" == x ]]; then export FUND_DETAIL_LEDGER_XLSX="$_RUN_ALL_CLI_VAL_FUND_DETAIL_LEDGER"; fi
if [[ "$_RUN_ALL_CLI_SET_MINISTRY_QUIET" == x ]]; then export MINISTRY_IMPORT_QUIET="$_RUN_ALL_CLI_VAL_MINISTRY_QUIET"; fi

# ---------------------------------------------------------------------------
# 하위 스크립트로 전달 (아직 비어 있으면 [편집] 기본값)
# ---------------------------------------------------------------------------
export FISCAL_YEAR="${FISCAL_YEAR:-$_RUN_ALL_DEFAULT_FISCAL_YEAR}"
export MINISTRY_LEDGER_XLSX="${MINISTRY_LEDGER_XLSX:-$_RUN_ALL_DEFAULT_MINISTRY_LEDGER}"
export FUND_SUMMARY_XLSX="${FUND_SUMMARY_XLSX:-$_RUN_ALL_DEFAULT_FUND_SUMMARY}"
export FUND_DETAIL_LEDGER_XLSX="${FUND_DETAIL_LEDGER_XLSX:-$_RUN_ALL_DEFAULT_FUND_DETAIL_LEDGER}"
export MINISTRY_IMPORT_QUIET="${MINISTRY_IMPORT_QUIET:-$_RUN_ALL_DEFAULT_MINISTRY_QUIET}"

DRY=()
if [[ "${1:-}" == "--dry-run" ]] || [[ "${1:-}" == "-n" ]]; then
  DRY=( --dry-run )
fi

echo "=== run_all_imports: (1) ministry org_balance  (2) fund activity fiscal-year replace ===" >&2
if [[ ${#DRY[@]} -gt 0 ]]; then
  echo "    mode=dry-run (DB 쓰기 없음: ministry 파싱만, fund activity 파싱·건수만)" >&2
else
  echo "    mode=live (org_balance + fund_activity_* 갱신)" >&2
fi
echo "    FISCAL_YEAR=$FISCAL_YEAR" >&2
echo "    MINISTRY_LEDGER_XLSX=${MINISTRY_LEDGER_XLSX:-<하위 스크립트 기본>}" >&2
echo "    FUND_SUMMARY_XLSX=${FUND_SUMMARY_XLSX:-<하위 스크립트 기본>}" >&2
echo "    FUND_DETAIL_LEDGER_XLSX=${FUND_DETAIL_LEDGER_XLSX:-<하위 스크립트 기본>}" >&2
echo "    MINISTRY_IMPORT_QUIET=$MINISTRY_IMPORT_QUIET" >&2
echo "" >&2

bash "$ROOT/scripts/import_ministry_org_balance.sh" "${DRY[@]+"${DRY[@]}"}"
echo "" >&2
bash "$ROOT/scripts/import_fund_activity_fiscal_year.sh" "${DRY[@]+"${DRY[@]}"}"

echo "" >&2
echo "=== run_all_imports: finished ===" >&2
