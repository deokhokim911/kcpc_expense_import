package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const fundActivitySummarySheetName = "Fund Activity Summary"

var fundActivityPeriodRE = regexp.MustCompile(`(?i)period\s+of\s+(\d{1,2}/\d{1,2}/\d{4})\s+to\s+(\d{1,2}/\d{1,2}/\d{4})`)

func parsePeriodFromReportLine(line string) (start, end time.Time, err error) {
	line = strings.TrimSpace(line)
	m := fundActivityPeriodRE.FindStringSubmatch(line)
	if m == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("no period range in line: %q", line)
	}
	const layout = "1/2/2006"
	start, err = time.ParseInLocation(layout, m[1], time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start date: %w", err)
	}
	end, err = time.ParseInLocation(layout, m[2], time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("end date: %w", err)
	}
	return start, end, nil
}

func findFundActivityHeaderRow(rows [][]string) int {
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}
		a := strings.TrimSpace(row[0])
		if !strings.EqualFold(a, "Fund") {
			continue
		}
		if len(row) > 1 && strings.Contains(strings.TrimSpace(row[1]), "Beginning") {
			return i
		}
	}
	return -1
}

func parseOptionalFloat(s string) (sql.NullFloat64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullFloat64{}, nil
	}
	s = strings.ReplaceAll(s, ",", "")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return sql.NullFloat64{}, err
	}
	return sql.NullFloat64{Float64: v, Valid: true}, nil
}

func loadFundActivitySummaryRows(path string) ([][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var sheet string
	for _, name := range f.GetSheetList() {
		if name == fundActivitySummarySheetName {
			sheet = name
			break
		}
	}
	if sheet == "" {
		return nil, fmt.Errorf("sheet %q not found in %s", fundActivitySummarySheetName, path)
	}
	return f.GetRows(sheet)
}

func runFundActivitySummaryImport(db *sql.DB, fiscalYear int, summaryPath, detailPath string, dryRun bool) error {
	if !dryRun && db == nil {
		return fmt.Errorf("database connection required unless -dry-run")
	}
	rows, err := loadFundActivitySummaryRows(summaryPath)
	if err != nil {
		return err
	}

	titleIdx := 1
	if len(rows) <= titleIdx {
		return fmt.Errorf("summary xlsx: missing title row")
	}
	var titleLine string
	if len(rows[titleIdx]) > 0 {
		titleLine = strings.TrimSpace(rows[titleIdx][0])
	}
	periodStart, periodEnd, err := parsePeriodFromReportLine(titleLine)
	if err != nil {
		return fmt.Errorf("summary period: %w", err)
	}

	if detailPath != "" {
		dRows, err := loadWorksheetRows(detailPath)
		if err != nil {
			return fmt.Errorf("detail xlsx: %w", err)
		}
		if len(dRows) < 2 {
			return fmt.Errorf("detail xlsx: too few rows")
		}
		dLine := ""
		if len(dRows[1]) > 0 {
			dLine = strings.TrimSpace(dRows[1][0])
		}
		ds, de, err := parsePeriodFromReportLine(dLine)
		if err != nil {
			return fmt.Errorf("detail period: %w", err)
		}
		if !periodStart.Equal(ds) || !periodEnd.Equal(de) {
			return fmt.Errorf("period mismatch: summary %s–%s vs detail %s–%s",
				periodStart.Format(time.DateOnly), periodEnd.Format(time.DateOnly),
				ds.Format(time.DateOnly), de.Format(time.DateOnly))
		}
	}

	hdr := findFundActivityHeaderRow(rows)
	if hdr < 0 {
		return fmt.Errorf("could not find Fund / Beginning Balance header row")
	}

	now := time.Now()
	summaryBase := filepath.Base(summaryPath)
	var detailBase sql.NullString
	if detailPath != "" {
		detailBase = sql.NullString{String: filepath.Base(detailPath), Valid: true}
	}

	type rowRec struct {
		fund    string
		nums    [8]sql.NullFloat64
		isTotal bool
	}
	var recs []rowRec
	for _, raw := range rows[hdr+1:] {
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
				return fmt.Errorf("row %q col %d: %w", fund, j+2, err)
			}
			nums[j] = n
		}
		recs = append(recs, rowRec{
			fund:    fund,
			nums:    nums,
			isTotal: isTotal,
		})
	}

	if len(recs) == 0 {
		return fmt.Errorf("no data rows after header")
	}

	if dryRun {
		fmt.Printf("[dry-run] fiscal_year=%d period=%s to %s\n", fiscalYear, periodStart.Format(time.DateOnly), periodEnd.Format(time.DateOnly))
		fmt.Printf("[dry-run] summary file=%s\n", summaryBase)
		if detailPath != "" {
			fmt.Printf("[dry-run] detail file=%s (period check OK)\n", detailBase.String)
		}
		fmt.Printf("[dry-run] would insert 1 import_run + %d summary_row(s):\n", len(recs))
		for i, r := range recs {
			fmt.Printf("[dry-run]   %d / %d %q total=%v\n", i+1, len(recs), r.fund, r.isTotal)
		}
		return nil
	}

	log.Printf("fund activity summary: starting transaction (%d fund rows, file=%s)", len(recs), summaryBase)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const qRun = `INSERT INTO public.fund_activity_import_run (
		fiscalyear, period_start, period_end,
		summary_imported_at, detail_imported_at,
		summary_source_filename, detail_source_filename,
		notes
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`

	var detailImportedAt sql.NullTime
	if detailPath != "" {
		detailImportedAt = sql.NullTime{Time: now, Valid: true}
	}

	var runID int
	err = tx.QueryRow(qRun,
		fiscalYear,
		periodStart,
		periodEnd,
		now,
		detailImportedAt,
		summaryBase,
		detailBase,
		sql.NullString{},
	).Scan(&runID)
	if err != nil {
		return fmt.Errorf("insert fund_activity_import_run: %w", err)
	}
	log.Printf("fund activity summary: inserted fund_activity_import_run id=%d", runID)

	const qRow = `INSERT INTO public.fund_activity_summary_row (
		run_id, fund_name,
		beginning_balance, income, expenses, net_income_expense,
		transfer, net_increase_decrease, ending_balance, beginning_of_fiscal_year_balance,
		is_total_row
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

	nRec := len(recs)
	for i, r := range recs {
		_, err := tx.Exec(qRow,
			runID,
			r.fund,
			r.nums[0], r.nums[1], r.nums[2], r.nums[3],
			r.nums[4], r.nums[5], r.nums[6], r.nums[7],
			r.isTotal,
		)
		if err != nil {
			return fmt.Errorf("insert summary row %q: %w", r.fund, err)
		}
		log.Printf("summary row %d / %d: %q (total=%v)", i+1, nRec, r.fund, r.isTotal)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("fund activity summary: committed run_id=%d total_rows=%d file=%s", runID, len(recs), summaryBase)
	return nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  %s -import-fund-summary -summary-xlsx <path> [-detail-xlsx <path>] [-fiscal-year N] [-dry-run]

  Imports Fund Activity Summary into fund_activity_import_run + fund_activity_summary_row only.
  Does not modify org_* tables.

  %s -import-fund-detail -detail-ledger-xlsx <path> (-run-id N | -match-run-by-period) [-dry-run]

  Imports General Ledger sheet into fund_activity_detail_line for an existing import run.

  %s -replace-fund-activity-year -fiscal-year N -summary-xlsx <path> -detail-ledger-xlsx <path> [-dry-run]

  Deletes all fund_activity_import_run (and CASCADE summary/detail) for that fiscal year, then loads both files.

Legacy org_balance import (ministry ledger only; off by default):
  %s -import-org-balance [-ministry-ledger-xlsx <path>] [-dry-run] [-quiet]

Default: no action (legacy ledger import is disabled).
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}
