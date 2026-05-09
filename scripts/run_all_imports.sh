#!/usr/bin/env bash
# 사역원 org_balance + Fund Activity(해당 연도 Summary·Detail 전체 재적재) 를 순서대로 실행.
#
# 기본 동작:
#   1) DB 연결 후 dry-run 으로 삭제·적재 예정 건수를 파일별로 표시
#   2) 사용자 확인(y/yes) 후 실제 적재
#
# 인자:
#   --dry-run, -n   미리보기만 하고 종료 (확인·적재 없음)
#   --yes, -y       미리보기 후 확인 프롬프트 생략하고 바로 적재
#
# 환경변수:
#   RUN_ALL_IMPORTS_YES=1   --yes 와 동일
#   기타 FISCAL_YEAR, MINISTRY_*, FUND_* 는 기존과 동일
#
# 설정 우선순위:
#   1) 실행 전 환경변수
#   2) run_all_imports.env (RUN_ALL_IMPORTS_ENV 로 경로 지정)
#   3) 스크립트 내 [편집] _RUN_ALL_DEFAULT_*

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

_PREVIEW_ONLY=false
_AUTO_YES=false
for _a in "$@"; do
  case "$_a" in
    --dry-run|-n) _PREVIEW_ONLY=true ;;
    --yes|-y) _AUTO_YES=true ;;
  esac
done
if [[ "${RUN_ALL_IMPORTS_YES:-}" == "1" ]]; then
  _AUTO_YES=true
fi

# 실행 전에 호출자가 넣은 값(비어 있지 않을 때만) — env 로드 후에도 우선 적용
# 주의: 셸에 MINISTRY_LEDGER_XLSX="" 처럼 빈 export 만 있으면 복구하지 않음 (env 파일 값 유지)
_RUN_ALL_PRE_FY="${FISCAL_YEAR-}"
_RUN_ALL_OVERRIDE_FY=false
[[ -n "${FISCAL_YEAR:-}" ]] && _RUN_ALL_OVERRIDE_FY=true

_RUN_ALL_PRE_MIN="${MINISTRY_LEDGER_XLSX-}"
_RUN_ALL_OVERRIDE_MIN=false
[[ -n "${MINISTRY_LEDGER_XLSX:-}" ]] && _RUN_ALL_OVERRIDE_MIN=true

_RUN_ALL_PRE_FS="${FUND_SUMMARY_XLSX-}"
_RUN_ALL_OVERRIDE_FS=false
[[ -n "${FUND_SUMMARY_XLSX:-}" ]] && _RUN_ALL_OVERRIDE_FS=true

_RUN_ALL_PRE_FD="${FUND_DETAIL_LEDGER_XLSX-}"
_RUN_ALL_OVERRIDE_FD=false
[[ -n "${FUND_DETAIL_LEDGER_XLSX:-}" ]] && _RUN_ALL_OVERRIDE_FD=true

_RUN_ALL_PRE_MQ="${MINISTRY_IMPORT_QUIET-}"
_RUN_ALL_OVERRIDE_MQ=false
[[ -n "${MINISTRY_IMPORT_QUIET:-}" ]] && _RUN_ALL_OVERRIDE_MQ=true

_RUN_ALL_DEFAULT_FISCAL_YEAR="${_RUN_ALL_DEFAULT_FISCAL_YEAR:-2026}"
_RUN_ALL_DEFAULT_MINISTRY_LEDGER="${_RUN_ALL_DEFAULT_MINISTRY_LEDGER:-}"
_RUN_ALL_DEFAULT_FUND_SUMMARY="${_RUN_ALL_DEFAULT_FUND_SUMMARY:-}"
_RUN_ALL_DEFAULT_FUND_DETAIL_LEDGER="${_RUN_ALL_DEFAULT_FUND_DETAIL_LEDGER:-}"
_RUN_ALL_DEFAULT_MINISTRY_QUIET="${_RUN_ALL_DEFAULT_MINISTRY_QUIET:-0}"

_ENV_FILE="${RUN_ALL_IMPORTS_ENV:-$SCRIPT_DIR/run_all_imports.env}"
if [[ -f "$_ENV_FILE" ]]; then
  echo "run_all_imports: loading $_ENV_FILE" >&2
  _ENV_CLEAN="$(mktemp)"
  tr -d '\r' < "$_ENV_FILE" > "$_ENV_CLEAN"
  set -a
  # shellcheck disable=SC1090
  source "$_ENV_CLEAN"
  set +a
  rm -f "$_ENV_CLEAN"
fi

if [[ "$_RUN_ALL_OVERRIDE_FY" == true ]]; then export FISCAL_YEAR="$_RUN_ALL_PRE_FY"; fi
if [[ "$_RUN_ALL_OVERRIDE_MIN" == true ]]; then export MINISTRY_LEDGER_XLSX="$_RUN_ALL_PRE_MIN"; fi
if [[ "$_RUN_ALL_OVERRIDE_FS" == true ]]; then export FUND_SUMMARY_XLSX="$_RUN_ALL_PRE_FS"; fi
if [[ "$_RUN_ALL_OVERRIDE_FD" == true ]]; then export FUND_DETAIL_LEDGER_XLSX="$_RUN_ALL_PRE_FD"; fi
if [[ "$_RUN_ALL_OVERRIDE_MQ" == true ]]; then export MINISTRY_IMPORT_QUIET="$_RUN_ALL_PRE_MQ"; fi

export FISCAL_YEAR="${FISCAL_YEAR:-$_RUN_ALL_DEFAULT_FISCAL_YEAR}"
export MINISTRY_LEDGER_XLSX="${MINISTRY_LEDGER_XLSX:-$_RUN_ALL_DEFAULT_MINISTRY_LEDGER}"
export FUND_SUMMARY_XLSX="${FUND_SUMMARY_XLSX:-$_RUN_ALL_DEFAULT_FUND_SUMMARY}"
export FUND_DETAIL_LEDGER_XLSX="${FUND_DETAIL_LEDGER_XLSX:-$_RUN_ALL_DEFAULT_FUND_DETAIL_LEDGER}"
export MINISTRY_IMPORT_QUIET="${MINISTRY_IMPORT_QUIET:-$_RUN_ALL_DEFAULT_MINISTRY_QUIET}"

# Go(main) DB 연결 — 미리보기·적재 모두 필요
if [[ -z "${DATABASE_URL:-}" ]]; then
  _pg_ok=true
  [[ -z "${PGHOST:-}" ]] && _pg_ok=false
  [[ -z "${PGPORT:-}" ]] && _pg_ok=false
  [[ -z "${PGUSER:-}" ]] && _pg_ok=false
  [[ -z "${PGPASSWORD:-}" ]] && _pg_ok=false
  [[ -z "${PGDATABASE:-}" ]] && _pg_ok=false
  if [[ "$_pg_ok" != true ]]; then
    echo "" >&2
    echo "run_all_imports: DB 연결 정보가 없습니다. 아래 중 하나를 설정하세요." >&2
    echo "  - DATABASE_URL=postgres://USER:PASSWORD@HOST:PORT/DBNAME?sslmode=disable" >&2
    echo "  - 또는 PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE" >&2
    echo "  → scripts/run_all_imports.env 에 넣거나, 실행 전에 export 하세요." >&2
    exit 1
  fi
fi

_run_preview() {
  local _log
  _log="$(mktemp)"
  trap 'rm -f "$_log"' RETURN

  echo "=== [1/2] 미리보기: 사역원 원장 (org_balance) ===" >&2
  MINISTRY_IMPORT_QUIET=1 bash "$ROOT/scripts/import_ministry_org_balance.sh" --dry-run 2>&1 | tee "$_log"
  echo "" >&2

  echo "=== [2/2] 미리보기: Fund Activity (fiscal-year replace) ===" >&2
  bash "$ROOT/scripts/import_fund_activity_fiscal_year.sh" --dry-run 2>&1 | tee -a "$_log"
  echo "" >&2

  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" >&2
  echo "  미리보기 요약 (DB 적재 전)" >&2
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" >&2
  echo "  회계연도 FISCAL_YEAR=$FISCAL_YEAR" >&2
  echo "" >&2
  echo "  [사역원] 파일: $(grep '^\[dry-run\] file_ministry_ledger=' "$_log" | tail -1 | sed 's/^[^=]*=//')" >&2
  echo "          삭제 예정: $(grep '^\[dry-run\] org_balance planned_delete' "$_log" | tail -1 | sed 's/.*count=//') 건 (public.org_balance, 해당 fiscalyear)" >&2
  echo "          적재 예정: $(grep '^\[dry-run\] org_balance planned_insert' "$_log" | tail -1 | sed 's/.*count=//') 건 (엑셀 파싱 기준)" >&2
  echo "" >&2
  echo "  [Fund Activity] Summary 파일: $(grep '^\[dry-run\] file_fund_summary=' "$_log" | tail -1 | sed 's/^[^=]*=//')" >&2
  echo "                  Detail 원장:  $(grep '^\[dry-run\] file_fund_detail_ledger=' "$_log" | tail -1 | sed 's/^[^=]*=//')" >&2
  echo "                  삭제 예정: import_run=$(grep '^\[dry-run\] fund_activity planned_delete' "$_log" | tail -1 | sed -n 's/.*import_run=\([0-9]*\).*/\1/p') 건," >&2
  echo "                             summary_row=$(grep '^\[dry-run\] fund_activity planned_delete' "$_log" | tail -1 | sed -n 's/.*summary_row=\([0-9]*\).*/\1/p') 건," >&2
  echo "                             detail_line=$(grep '^\[dry-run\] fund_activity planned_delete' "$_log" | tail -1 | sed -n 's/.*detail_line=\([0-9]*\).*/\1/p') 건 (CASCADE)" >&2
  echo "                  적재 예정: import_run=$(grep '^\[dry-run\] fund_activity planned_insert' "$_log" | tail -1 | sed -n 's/.*import_run=\([0-9]*\).*/\1/p') 건," >&2
  echo "                             summary_row=$(grep '^\[dry-run\] fund_activity planned_insert' "$_log" | tail -1 | sed -n 's/.*summary_row=\([0-9]*\).*/\1/p') 건," >&2
  echo "                             detail_line=$(grep '^\[dry-run\] fund_activity planned_insert' "$_log" | tail -1 | sed -n 's/.*detail_line=\([0-9]*\).*/\1/p') 건" >&2
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" >&2
}

echo "=== run_all_imports ===" >&2
echo "    FISCAL_YEAR=$FISCAL_YEAR" >&2
echo "    MINISTRY_LEDGER_XLSX=${MINISTRY_LEDGER_XLSX:-<기본>}" >&2
echo "    FUND_SUMMARY_XLSX=${FUND_SUMMARY_XLSX:-<기본>}" >&2
echo "    FUND_DETAIL_LEDGER_XLSX=${FUND_DETAIL_LEDGER_XLSX:-<기본>}" >&2
echo "" >&2

if [[ "$_PREVIEW_ONLY" == true ]]; then
  echo "    mode=--dry-run 전용 (적재·확인 없음)" >&2
  _run_preview
  echo "=== run_all_imports: 미리보기만 종료 ===" >&2
  exit 0
fi

_run_preview

if [[ "$_AUTO_YES" != true ]]; then
  echo "" >&2
  read -r -p "위 내용으로 DB에 적재하시겠습니까? [y/N]: " _confirm || true
  case "${_confirm:-}" in
    y|Y|yes|YES) ;;
    *)
      echo "취소되었습니다. (적재하지 않음)" >&2
      exit 0
      ;;
  esac
else
  echo "RUN_ALL_IMPORTS_YES=1 또는 --yes: 확인 생략 후 적재합니다." >&2
fi

echo "" >&2
echo "=== 실제 적재: (1) ministry org_balance ===" >&2
bash "$ROOT/scripts/import_ministry_org_balance.sh"
echo "" >&2
echo "=== 실제 적재: (2) fund activity fiscal-year ===" >&2
bash "$ROOT/scripts/import_fund_activity_fiscal_year.sh"

echo "" >&2
echo "=== run_all_imports: 적재 완료 ===" >&2
