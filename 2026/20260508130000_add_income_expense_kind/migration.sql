-- fund_activity_detail_line: Income/Expense 구간 분류 (Name 열 Number - Text 파싱)

ALTER TABLE "fund_activity_detail_line"
    ADD COLUMN "income_expense_kind" VARCHAR(10),
    ADD COLUMN "account_code" VARCHAR(20);

CREATE INDEX "fund_activity_detail_line_run_fund_kind_idx"
    ON "fund_activity_detail_line" ("run_id", "fund_name", "income_expense_kind");

ALTER TABLE "fund_activity_detail_line"
    ADD CONSTRAINT "fund_activity_detail_line_income_expense_kind_check"
    CHECK ("income_expense_kind" IS NULL OR "income_expense_kind" IN ('Income', 'Expense'));
