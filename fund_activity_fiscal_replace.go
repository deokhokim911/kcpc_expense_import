package main

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"
)

func countFundActivityDataForFiscalYear(db *sql.DB, fiscalYear int) (runs, summaryRows, detailLines int, err error) {
	err = db.QueryRow(`SELECT COUNT(*) FROM public.fund_activity_import_run WHERE fiscalyear = $1`, fiscalYear).Scan(&runs)
	if err != nil {
		return 0, 0, 0, err
	}
	err = db.QueryRow(`
		SELECT COUNT(*) FROM public.fund_activity_summary_row s
		INNER JOIN public.fund_activity_import_run i ON s.run_id = i.id
		WHERE i.fiscalyear = $1`, fiscalYear).Scan(&summaryRows)
	if err != nil {
		return 0, 0, 0, err
	}
	err = db.QueryRow(`
		SELECT COUNT(*) FROM public.fund_activity_detail_line d
		INNER JOIN public.fund_activity_import_run i ON d.run_id = i.id
		WHERE i.fiscalyear = $1`, fiscalYear).Scan(&detailLines)
	if err != nil {
		return 0, 0, 0, err
	}
	return runs, summaryRows, detailLines, nil
}

// runFundActivityFiscalYearReplace deletes all fund_activity_import_run rows for the given fiscal year
// (CASCADE removes fund_activity_summary_row and fund_activity_detail_line), then inserts one new run
// with parsed Summary + General Ledger detail from the given files.
func runFundActivityFiscalYearReplace(db *sql.DB, fiscalYear int, summaryPath, detailLedgerPath string, dryRun bool) error {
	if !dryRun && db == nil {
		return fmt.Errorf("database connection required unless -dry-run")
	}
	if detailLedgerPath == "" {
		return fmt.Errorf("detail ledger path is required for full fiscal replace")
	}

	summaryRows, err := loadFundActivitySummaryRows(summaryPath)
	if err != nil {
		return err
	}
	titleIdx := 1
	if len(summaryRows) <= titleIdx {
		return fmt.Errorf("summary xlsx: missing title row")
	}
	var titleLine string
	if len(summaryRows[titleIdx]) > 0 {
		titleLine = strings.TrimSpace(summaryRows[titleIdx][0])
	}
	periodStart, periodEnd, err := parsePeriodFromReportLine(titleLine)
	if err != nil {
		return fmt.Errorf("summary period: %w", err)
	}

	glRows, err := loadGeneralLedgerRows(detailLedgerPath)
	if err != nil {
		return fmt.Errorf("detail ledger: %w", err)
	}
	if len(glRows) < 3 {
		return fmt.Errorf("general ledger xlsx: too few rows")
	}
	glTitle := ""
	if len(glRows) > 1 && len(glRows[1]) > 0 {
		glTitle = strings.TrimSpace(glRows[1][0])
	}
	ds, de, err := parsePeriodFromReportLine(glTitle)
	if err != nil {
		return fmt.Errorf("ledger period: %w", err)
	}
	if !periodStart.Equal(ds) || !periodEnd.Equal(de) {
		return fmt.Errorf("period mismatch: summary %s–%s vs ledger %s–%s",
			periodStart.Format(time.DateOnly), periodEnd.Format(time.DateOnly),
			ds.Format(time.DateOnly), de.Format(time.DateOnly))
	}

	hdrSum := findFundActivityHeaderRow(summaryRows)
	if hdrSum < 0 {
		return fmt.Errorf("could not find Fund / Beginning Balance header row in summary")
	}

	type rowRec struct {
		fund    string
		nums    [8]sql.NullFloat64
		isTotal bool
	}
	var recs []rowRec
	for _, raw := range summaryRows[hdrSum+1:] {
		if len(raw) == 0 || strings.TrimSpace(raw[0]) == "" {
			continue
		}
		fund := strings.TrimSpace(raw[0])
		isTotal := strings.EqualFold(fund, "Total")
		padded := padRow(raw, 9)
		var nums [8]sql.NullFloat64
		for j := 0; j < 8; j++ {
			n, err := parseOptionalFloat(padded[1+j])
			if err != nil {
				return fmt.Errorf("summary row %q col %d: %w", fund, j+2, err)
			}
			nums[j] = n
		}
		recs = append(recs, rowRec{fund: fund, nums: nums, isTotal: isTotal})
	}
	if len(recs) == 0 {
		return fmt.Errorf("no data rows after summary header")
	}

	hdrGL := findGeneralLedgerHeaderRow(glRows)
	if hdrGL < 0 {
		return fmt.Errorf("could not find General Ledger header row (Name / Date)")
	}
	detailLines, err := parseGeneralLedgerDetailLines(glRows, hdrGL)
	if err != nil {
		return err
	}

	summaryBase := filepath.Base(summaryPath)
	detailBase := filepath.Base(detailLedgerPath)

	if dryRun {
		if db != nil {
			runs, sCt, dCt, err := countFundActivityDataForFiscalYear(db, fiscalYear)
			if err != nil {
				return fmt.Errorf("count existing data: %w", err)
			}
			fmt.Printf("[dry-run] fiscal_year=%d existing DB: import_run=%d summary_rows=%d detail_lines=%d (would DELETE via CASCADE)\n",
				fiscalYear, runs, sCt, dCt)
			fmt.Printf("[dry-run] fund_activity planned_delete fiscalyear=%d import_run=%d summary_row=%d detail_line=%d\n",
				fiscalYear, runs, sCt, dCt)
		} else {
			fmt.Printf("[dry-run] fiscal_year=%d (no DB: cannot count existing rows)\n", fiscalYear)
		}
		fmt.Printf("[dry-run] file_fund_summary=%s\n", summaryBase)
		fmt.Printf("[dry-run] file_fund_detail_ledger=%s\n", detailBase)
		fmt.Printf("[dry-run] period=%s to %s\n", periodStart.Format(time.DateOnly), periodEnd.Format(time.DateOnly))
		fmt.Printf("[dry-run] summary=%s ledger=%s\n", summaryBase, detailBase)
		fmt.Printf("[dry-run] fund_activity planned_insert import_run=1 summary_row=%d detail_line=%d\n", len(recs), len(detailLines))
		fmt.Printf("[dry-run] would insert 1 import_run + %d summary_row(s) + %d detail_line(s)\n", len(recs), len(detailLines))
		return nil
	}

	runs, sCt, dCt, err := countFundActivityDataForFiscalYear(db, fiscalYear)
	if err != nil {
		return fmt.Errorf("count existing data: %w", err)
	}
	log.Printf("fund activity fiscal replace: fiscal_year=%d deleting existing import_run=%d (summary_rows=%d detail_lines=%d CASCADE)",
		fiscalYear, runs, sCt, dCt)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`DELETE FROM public.fund_activity_import_run WHERE fiscalyear = $1`, fiscalYear)
	if err != nil {
		return fmt.Errorf("delete fund_activity_import_run for fiscal year: %w", err)
	}
	nDel, _ := res.RowsAffected()
	log.Printf("fund activity fiscal replace: removed %d import_run row(s) for fiscalyear=%d", nDel, fiscalYear)

	now := time.Now()
	const qRun = `INSERT INTO public.fund_activity_import_run (
		fiscalyear, period_start, period_end,
		summary_imported_at, detail_imported_at,
		summary_source_filename, detail_source_filename,
		notes
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`

	var runID int
	err = tx.QueryRow(qRun,
		fiscalYear,
		periodStart,
		periodEnd,
		now,
		now,
		summaryBase,
		detailBase,
		sql.NullString{String: "replace-fund-activity-year", Valid: true},
	).Scan(&runID)
	if err != nil {
		return fmt.Errorf("insert fund_activity_import_run: %w", err)
	}
	log.Printf("fund activity fiscal replace: new fund_activity_import_run id=%d", runID)

	const qSum = `INSERT INTO public.fund_activity_summary_row (
		run_id, fund_name,
		beginning_balance, income, expenses, net_income_expense,
		transfer, net_increase_decrease, ending_balance, beginning_of_fiscal_year_balance,
		is_total_row
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

	nRec := len(recs)
	for i, r := range recs {
		_, err := tx.Exec(qSum,
			runID,
			r.fund,
			r.nums[0], r.nums[1], r.nums[2], r.nums[3],
			r.nums[4], r.nums[5], r.nums[6], r.nums[7],
			r.isTotal,
		)
		if err != nil {
			return fmt.Errorf("insert summary row %q: %w", r.fund, err)
		}
		log.Printf("fiscal replace summary %d / %d: %q (total=%v)", i+1, nRec, r.fund, r.isTotal)
	}

	const qDet = `INSERT INTO public.fund_activity_detail_line (
		run_id, fund_name, line_date, transaction_number, transaction_type,
		contact, memo, reference_number, note,
		debit, credit, amount, balance,
		account_section, row_label, source_row_order
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`

	totalLines := len(detailLines)
	const progressEvery = 50
	for i, ln := range detailLines {
		_, err := tx.Exec(qDet,
			runID,
			ln.fundName,
			ln.lineDate,
			ln.transactionNumber,
			ln.transactionType,
			ln.contact,
			ln.memo,
			ln.referenceNumber,
			ln.note,
			ln.debit,
			ln.credit,
			ln.amount,
			ln.balance,
			ln.accountSection,
			ln.rowLabel,
			ln.sourceRowOrder,
		)
		if err != nil {
			return fmt.Errorf("insert detail fund=%q order=%d: %w", ln.fundName, ln.sourceRowOrder, err)
		}
		n := i + 1
		if n%progressEvery == 0 || n == totalLines {
			log.Printf("fiscal replace detail %d / %d (latest fund=%q)", n, totalLines, truncateRunes(ln.fundName, 48))
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("fund activity fiscal replace: committed run_id=%d summary=%d detail=%d files=%s + %s",
		runID, len(recs), len(detailLines), summaryBase, detailBase)
	return nil
}
