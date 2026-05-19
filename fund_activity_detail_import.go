package main

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

const (
	generalLedgerSheetName = "General Ledger"
	glMinCols              = 16

	incomeExpenseKindIncome  = "Income"
	incomeExpenseKindExpense = "Expense"
)

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	r := []rune(s)
	return string(r[:maxRunes]) + "…"
}

func loadGeneralLedgerRows(path string) ([][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	for _, name := range f.GetSheetList() {
		if name == generalLedgerSheetName {
			return f.GetRows(name)
		}
	}
	return nil, fmt.Errorf("sheet %q not found in %s", generalLedgerSheetName, path)
}

func findGeneralLedgerHeaderRow(rows [][]string) int {
	for i, row := range rows {
		if len(row) < 3 {
			continue
		}
		if strings.TrimSpace(row[0]) == "Name" && strings.TrimSpace(row[1]) == "Date" {
			return i
		}
	}
	return -1
}

func parseOptionalDateMMDDYYYY(s string) (sql.NullTime, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullTime{}, nil
	}
	const layout = "01/02/2006"
	t, err := time.ParseInLocation(layout, strings.ReplaceAll(s, " ", ""), time.Local)
	if err != nil {
		return sql.NullTime{}, err
	}
	return sql.NullTime{Time: t, Valid: true}, nil
}

func parseOptionalString(s string) sql.NullString {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func isGLCellEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// isNumberText reports whether s is "Number - Text" (first '-' splits account code from label).
func isNumberText(s string) bool {
	_, ok := parseNumberText(s)
	return ok
}

func parseNumberText(s string) (code string, ok bool) {
	s = strings.TrimSpace(s)
	idx := strings.Index(s, "-")
	if idx < 0 {
		return "", false
	}
	code = strings.TrimSpace(s[:idx])
	if code == "" {
		return "", false
	}
	if _, err := strconv.Atoi(code); err != nil {
		return "", false
	}
	return code, true
}

// classifyAccountCode maps QB account number to Income/Expense kind, or "" if no rule matches.
func classifyAccountCode(code string) string {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "1865") {
		return incomeExpenseKindExpense
	}
	n, err := strconv.Atoi(code)
	if err != nil {
		return ""
	}
	if n >= 4300 && n <= 4510 {
		return incomeExpenseKindIncome
	}
	if n >= 5180 && n <= 9010 {
		return incomeExpenseKindExpense
	}
	return ""
}

// detailLine represents one row to insert (fund_name and income_expense_kind always set).
type detailLine struct {
	fundName          string
	incomeExpenseKind string
	accountCode       sql.NullString
	lineDate          sql.NullTime
	transactionNumber sql.NullString
	transactionType   sql.NullString
	contact           sql.NullString
	memo              sql.NullString
	referenceNumber   sql.NullString
	note              sql.NullString
	debit             sql.NullFloat64
	credit            sql.NullFloat64
	amount            sql.NullFloat64
	balance           sql.NullFloat64
	accountSection    sql.NullString
	rowLabel          sql.NullString
	sourceRowOrder    int
}

func parseGeneralLedgerDetailLines(rows [][]string, headerIdx int) ([]detailLine, error) {
	var (
		activeKind     string
		accountSection string
		accountCode    string
		out            []detailLine
		order          int
	)

	for _, raw := range rows[headerIdx+1:] {
		rec := padRow(raw, glMinCols)
		order++

		name := strings.TrimSpace(rec[0])
		fund := strings.TrimSpace(rec[8])
		debitCell := rec[12]
		creditCell := rec[13]

		if strings.HasPrefix(name, "Total for") {
			continue
		}

		// Rule 1–4: account section header (Name only; Fund/Debit/Credit empty).
		if name != "" && isGLCellEmpty(fund) && isGLCellEmpty(debitCell) && isGLCellEmpty(creditCell) {
			if !isNumberText(name) {
				continue
			}
			code, ok := parseNumberText(name)
			if !ok {
				continue
			}
			if kind := classifyAccountCode(code); kind != "" {
				activeKind = kind
				accountSection = name
				accountCode = code
			}
			continue
		}

		if fund == "" {
			continue
		}

		if strings.EqualFold(name, "Beginning Balance") {
			continue
		}

		if activeKind == "" {
			continue
		}

		lineDate, err := parseOptionalDateMMDDYYYY(rec[1])
		if err != nil {
			return nil, fmt.Errorf("source_row_order=%d fund=%q date=%q: %w", order, fund, rec[1], err)
		}

		d := detailLine{
			fundName:          fund,
			incomeExpenseKind: activeKind,
			lineDate:          lineDate,
			transactionNumber: parseOptionalString(rec[2]),
			transactionType:   parseOptionalString(rec[3]),
			contact:           parseOptionalString(rec[4]),
			memo:              parseOptionalString(rec[5]),
			referenceNumber:   parseOptionalString(rec[6]),
			note:              parseOptionalString(rec[7]),
			sourceRowOrder:    order,
		}

		if accountSection != "" {
			d.accountSection = sql.NullString{String: accountSection, Valid: true}
		}
		if accountCode != "" {
			d.accountCode = sql.NullString{String: accountCode, Valid: true}
		}
		if name != "" {
			d.rowLabel = sql.NullString{String: name, Valid: true}
		}

		var errAmt error
		d.debit, errAmt = parseOptionalFloat(rec[12])
		if errAmt != nil {
			return nil, fmt.Errorf("row order %d debit %q: %w", order, rec[12], errAmt)
		}
		d.credit, errAmt = parseOptionalFloat(rec[13])
		if errAmt != nil {
			return nil, fmt.Errorf("row order %d credit %q: %w", order, rec[13], errAmt)
		}
		d.amount, errAmt = parseOptionalFloat(rec[14])
		if errAmt != nil {
			return nil, fmt.Errorf("row order %d amount %q: %w", order, rec[14], errAmt)
		}
		d.balance, errAmt = parseOptionalFloat(rec[15])
		if errAmt != nil {
			return nil, fmt.Errorf("row order %d balance %q: %w", order, rec[15], errAmt)
		}

		out = append(out, d)
	}

	return out, nil
}

func countDetailLinesByKind(lines []detailLine) (income, expense int) {
	for _, ln := range lines {
		switch ln.incomeExpenseKind {
		case incomeExpenseKindIncome:
			income++
		case incomeExpenseKindExpense:
			expense++
		}
	}
	return income, expense
}

func lookupRunIDByPeriod(db *sql.DB, start, end time.Time) (int, error) {
	var id int
	err := db.QueryRow(
		`SELECT id FROM public.fund_activity_import_run WHERE period_start = $1 AND period_end = $2 ORDER BY id DESC LIMIT 1`,
		start, end,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("no fund_activity_import_run for period %s–%s: %w", start.Format(time.DateOnly), end.Format(time.DateOnly), err)
	}
	return id, nil
}

func runFundActivityDetailImport(db *sql.DB, ledgerPath string, runID int, matchRunByPeriod bool, dryRun bool) error {
	if !dryRun && db == nil {
		return fmt.Errorf("database connection required unless -dry-run")
	}

	rows, err := loadGeneralLedgerRows(ledgerPath)
	if err != nil {
		return err
	}

	if len(rows) < 3 {
		return fmt.Errorf("general ledger xlsx: too few rows")
	}

	titleLine := ""
	if len(rows) > 1 && len(rows[1]) > 0 {
		titleLine = strings.TrimSpace(rows[1][0])
	}
	periodStart, periodEnd, err := parsePeriodFromReportLine(titleLine)
	if err != nil {
		return fmt.Errorf("ledger period line: %w", err)
	}

	hdr := findGeneralLedgerHeaderRow(rows)
	if hdr < 0 {
		return fmt.Errorf("could not find General Ledger header row (Name / Date)")
	}

	lines, err := parseGeneralLedgerDetailLines(rows, hdr)
	if err != nil {
		return err
	}

	resolvedRunID := runID
	switch {
	case resolvedRunID > 0:
		break
	case matchRunByPeriod:
		if db == nil {
			return fmt.Errorf("database connection required for -match-run-by-period")
		}
		id, err := lookupRunIDByPeriod(db, periodStart, periodEnd)
		if err != nil {
			return err
		}
		resolvedRunID = id
	default:
		return fmt.Errorf("set -run-id <id> (existing fund_activity_import_run) or -match-run-by-period")
	}

	base := filepath.Base(ledgerPath)

	if dryRun {
		incomeN, expenseN := countDetailLinesByKind(lines)
		fmt.Printf("[dry-run] detail file=%s period=%s to %s\n", base, periodStart.Format(time.DateOnly), periodEnd.Format(time.DateOnly))
		fmt.Printf("[dry-run] run_id=%d parsed detail lines=%d (income=%d expense=%d, no DB writes)\n", resolvedRunID, len(lines), incomeN, expenseN)
		return nil
	}

	totalLines := len(lines)
	log.Printf("fund activity detail: starting run_id=%d lines=%d file=%s", resolvedRunID, totalLines, base)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	log.Printf("fund activity detail: deleting existing rows for run_id=%d ...", resolvedRunID)
	if _, err := tx.Exec(`DELETE FROM public.fund_activity_detail_line WHERE run_id = $1`, resolvedRunID); err != nil {
		return fmt.Errorf("delete existing detail lines: %w", err)
	}
	log.Printf("fund activity detail: inserting %d rows ...", totalLines)

	const qIns = `INSERT INTO public.fund_activity_detail_line (
		run_id, fund_name, income_expense_kind, account_code,
		line_date, transaction_number, transaction_type,
		contact, memo, reference_number, note,
		debit, credit, amount, balance,
		account_section, row_label, source_row_order
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`

	const progressEvery = 50
	for i, ln := range lines {
		_, err := tx.Exec(qIns,
			resolvedRunID,
			ln.fundName,
			ln.incomeExpenseKind,
			ln.accountCode,
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
			log.Printf("fund activity detail: inserted %d / %d (latest fund=%q)", n, totalLines, truncateRunes(ln.fundName, 48))
		}
	}

	log.Printf("fund activity detail: updating fund_activity_import_run metadata ...")
	now := time.Now()
	if _, err := tx.Exec(
		`UPDATE public.fund_activity_import_run SET detail_imported_at = $1, detail_source_filename = $2 WHERE id = $3`,
		now, base, resolvedRunID,
	); err != nil {
		return fmt.Errorf("update import_run detail metadata: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("fund activity detail: committed run_id=%d total_lines=%d file=%s", resolvedRunID, totalLines, base)
	return nil
}
