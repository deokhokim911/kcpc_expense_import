-- 사역원·부서별 지출 집계 스냅샷 (적재 시 INSERT — 기존 org_balance / org_budget 무변경)
-- 문서: 2026/사역원_부서별_지출_집계_적재_개발방안.md §14

-- ENUM (재실행 시 duplicate_object 무시)
DO $$
BEGIN
    CREATE TYPE public.org_expense_agg_level AS ENUM ('ministry', 'department');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.org_balance_import_run (
    id               SERIAL PRIMARY KEY,
    fiscalyear       INTEGER NOT NULL,
    period_start     DATE,
    period_end       DATE,
    source_filename  VARCHAR(500) NOT NULL,
    imported_at      TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    detail_row_count INTEGER NOT NULL DEFAULT 0,
    notes            VARCHAR(2000)
);

CREATE INDEX IF NOT EXISTS org_balance_import_run_fiscalyear_idx
    ON public.org_balance_import_run (fiscalyear);

CREATE INDEX IF NOT EXISTS org_balance_import_run_fiscalyear_imported_at_idx
    ON public.org_balance_import_run (fiscalyear, imported_at DESC);

CREATE TABLE IF NOT EXISTS public.org_expense_summary_row (
    id                SERIAL PRIMARY KEY,
    run_id            INTEGER NOT NULL
        REFERENCES public.org_balance_import_run (id) ON DELETE CASCADE,
    fiscalyear        INTEGER NOT NULL,
    aggregation_level public.org_expense_agg_level NOT NULL,

    ministry_id       INTEGER NOT NULL,
    ministry_name     VARCHAR(500),

    department_code   INTEGER,
    department_name   VARCHAR(500),

    total_debit       DECIMAL(20, 6) NOT NULL DEFAULT 0,
    total_credit      DECIMAL(20, 6) NOT NULL DEFAULT 0,
    total_expense     DECIMAL(20, 6) NOT NULL DEFAULT 0,

    transaction_count INTEGER NOT NULL DEFAULT 0,
    budget_amount     DECIMAL(20, 6),
    remaining_budget  DECIMAL(20, 6),

    computed_at       TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT org_expense_summary_row_uq UNIQUE (
        run_id, aggregation_level, ministry_id, department_code
    )
);

CREATE INDEX IF NOT EXISTS org_expense_summary_row_run_ministry_idx
    ON public.org_expense_summary_row (run_id, ministry_id);

CREATE INDEX IF NOT EXISTS org_expense_summary_row_run_dept_idx
    ON public.org_expense_summary_row (run_id, ministry_id, department_code)
    WHERE aggregation_level = 'department';
