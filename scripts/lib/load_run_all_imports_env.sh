# scripts/lib/load_run_all_imports_env.sh
# 하위 import 스크립트에서 source — run_all_imports.env (CRLF 안전) 로드
#
#   source "$SCRIPT_DIR/lib/load_run_all_imports_env.sh"
#
# 환경변수:
#   RUN_ALL_IMPORTS_ENV — env 파일 경로 (기본: scripts/run_all_imports.env)
#
# 엑셀 경로 (MINISTRY_LEDGER_XLSX 등):
#   - 파일명만: GeneralLedger.xlsx  →  {repo}/current/GeneralLedger.xlsx
#   - current/ 접두: current/Foo.xlsx
#   - 절대 경로: /path/to/file.xlsx (그대로)

if [[ -n "${_KCPC_IMPORT_ENV_LOADED:-}" ]]; then
  return 0 2>/dev/null || true
fi

_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_KCPC_REPO_ROOT="$(cd "$_LIB_DIR/../.." && pwd)"
_ENV_FILE="${RUN_ALL_IMPORTS_ENV:-$_LIB_DIR/../run_all_imports.env}"

if [[ -f "$_ENV_FILE" ]]; then
  echo "import: loading $_ENV_FILE" >&2
  _ENV_CLEAN="$(mktemp)"
  tr -d '\r' < "$_ENV_FILE" > "$_ENV_CLEAN"
  set -a
  # shellcheck disable=SC1090
  source "$_ENV_CLEAN"
  set +a
  rm -f "$_ENV_CLEAN"
fi

_KCPC_IMPORT_ENV_LOADED=1

# repo/current/ 기준으로 env 의 파일명·상대 경로를 절대 경로로 변환
kcpc_resolve_import_xlsx() {
  local raw="${1:-}"
  if [[ -z "$raw" ]]; then
    printf ''
    return 0
  fi
  raw="${raw#"${raw%%[![:space:]]*}"}"
  raw="${raw%"${raw##*[![:space:]]}"}"
  if [[ -z "$raw" ]]; then
    printf ''
    return 0
  fi
  if [[ "$raw" == /* ]]; then
    printf '%s' "$raw"
    return 0
  fi
  raw="${raw#./}"
  if [[ "$raw" == current/* ]]; then
    printf '%s' "$_KCPC_REPO_ROOT/$raw"
    return 0
  fi
  printf '%s' "$_KCPC_REPO_ROOT/current/$raw"
}

kcpc_resolve_import_env_paths() {
  if [[ -n "${MINISTRY_LEDGER_XLSX:-}" ]]; then
    export MINISTRY_LEDGER_XLSX="$(kcpc_resolve_import_xlsx "$MINISTRY_LEDGER_XLSX")"
  fi
  if [[ -n "${FUND_SUMMARY_XLSX:-}" ]]; then
    export FUND_SUMMARY_XLSX="$(kcpc_resolve_import_xlsx "$FUND_SUMMARY_XLSX")"
  fi
  if [[ -n "${FUND_DETAIL_LEDGER_XLSX:-}" ]]; then
    export FUND_DETAIL_LEDGER_XLSX="$(kcpc_resolve_import_xlsx "$FUND_DETAIL_LEDGER_XLSX")"
  fi
  if [[ -n "${FUND_DETAIL_XLSX:-}" ]]; then
    export FUND_DETAIL_XLSX="$(kcpc_resolve_import_xlsx "$FUND_DETAIL_XLSX")"
  fi
}

kcpc_check_import_db_env() {
  if [[ -n "${DATABASE_URL:-}" ]]; then
    return 0
  fi
  if [[ -n "${PGHOST:-}" && -n "${PGPORT:-}" && -n "${PGUSER:-}" && -n "${PGPASSWORD:-}" && -n "${PGDATABASE:-}" ]]; then
    return 0
  fi
  echo "" >&2
  echo "import: DB 연결 정보가 없습니다." >&2
  echo "  scripts/run_all_imports.env 에 DATABASE_URL 또는 PG* 를 설정하거나," >&2
  echo "  실행 전에 export DATABASE_URL=... 하세요." >&2
  return 1
}
