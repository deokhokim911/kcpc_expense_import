-- org_expense_summary 스냅샷 테이블 제거 (Go 적재 로직 복원 시)
-- 문서: 2026/사역원_부서별_지출_집계_적재_개발방안.md §14.3.5
--
-- 적용:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
--     -f 2026/20260606120000_add_org_expense_summary/rollback.sql
--
-- 주의: org_expense_summary_row / org_balance_import_run 데이터가 삭제됩니다.
--       org_balance / org_budget / fund_activity_* 는 변경하지 않습니다.

DROP TABLE IF EXISTS public.org_expense_summary_row CASCADE;
DROP TABLE IF EXISTS public.org_balance_import_run CASCADE;
DROP TYPE IF EXISTS public.org_expense_agg_level;
