-- General Ledger(Special web posting) Detail 라인. 적재는 별도 프로세서/파서에서 수행.

CREATE TABLE "fund_activity_detail_line" (
    "id" SERIAL NOT NULL,
    "run_id" INTEGER NOT NULL,
    "fund_name" VARCHAR(500) NOT NULL,
    "line_date" DATE,
    "transaction_number" VARCHAR(100),
    "transaction_type" VARCHAR(100),
    "contact" VARCHAR(500),
    "memo" TEXT,
    "reference_number" VARCHAR(500),
    "note" TEXT,
    "debit" DECIMAL(20,6),
    "credit" DECIMAL(20,6),
    "amount" DECIMAL(20,6),
    "balance" DECIMAL(20,6),
    "account_section" VARCHAR(500),
    "row_label" VARCHAR(500),
    "source_row_order" INTEGER,

    CONSTRAINT "fund_activity_detail_line_pkey" PRIMARY KEY ("id")
);

CREATE INDEX "fund_activity_detail_line_run_id_idx" ON "fund_activity_detail_line"("run_id");

CREATE INDEX "fund_activity_detail_line_run_id_fund_name_idx" ON "fund_activity_detail_line"("run_id", "fund_name");

ALTER TABLE "fund_activity_detail_line" ADD CONSTRAINT "fund_activity_detail_line_run_id_fkey" FOREIGN KEY ("run_id") REFERENCES "fund_activity_import_run"("id") ON DELETE CASCADE ON UPDATE CASCADE;
