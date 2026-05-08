-- Fund Activity Summary 스냅샷(년도별 특별) — import run + summary 행
-- 적재는 앱이 아닌 별도 파서에서 수행.

CREATE TABLE "fund_activity_import_run" (
    "id" SERIAL NOT NULL,
    "fiscalyear" INTEGER NOT NULL,
    "period_start" DATE NOT NULL,
    "period_end" DATE NOT NULL,
    "summary_imported_at" TIMESTAMP(3),
    "detail_imported_at" TIMESTAMP(3),
    "summary_source_filename" VARCHAR(500),
    "detail_source_filename" VARCHAR(500),
    "notes" VARCHAR(2000),
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "fund_activity_import_run_pkey" PRIMARY KEY ("id")
);

CREATE TABLE "fund_activity_summary_row" (
    "id" SERIAL NOT NULL,
    "run_id" INTEGER NOT NULL,
    "fund_name" VARCHAR(500) NOT NULL,
    "beginning_balance" DECIMAL(20,6),
    "income" DECIMAL(20,6),
    "expenses" DECIMAL(20,6),
    "net_income_expense" DECIMAL(20,6),
    "transfer" DECIMAL(20,6),
    "net_increase_decrease" DECIMAL(20,6),
    "ending_balance" DECIMAL(20,6),
    "beginning_of_fiscal_year_balance" DECIMAL(20,6),
    "is_total_row" BOOLEAN NOT NULL DEFAULT false,

    CONSTRAINT "fund_activity_summary_row_pkey" PRIMARY KEY ("id")
);

CREATE INDEX "fund_activity_import_run_fiscalyear_idx" ON "fund_activity_import_run"("fiscalyear");

CREATE INDEX "fund_activity_import_run_period_start_period_end_idx" ON "fund_activity_import_run"("period_start", "period_end");

CREATE INDEX "fund_activity_summary_row_run_id_idx" ON "fund_activity_summary_row"("run_id");

CREATE INDEX "fund_activity_summary_row_run_id_fund_name_idx" ON "fund_activity_summary_row"("run_id", "fund_name");

ALTER TABLE "fund_activity_summary_row" ADD CONSTRAINT "fund_activity_summary_row_run_id_fkey" FOREIGN KEY ("run_id") REFERENCES "fund_activity_import_run"("id") ON DELETE CASCADE ON UPDATE CASCADE;
