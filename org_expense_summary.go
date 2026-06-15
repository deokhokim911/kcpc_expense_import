package main

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

const (
	orgExpenseAggDepartment = "department"
	orgExpenseAggMinistry   = "ministry"
)

type deptExpenseKey struct {
	ministryID int
	deptCode   int
}

type expenseDeptAgg struct {
	ministryID       int
	ministryName     string
	departmentCode   int
	departmentName   string
	totalDebit       float64
	totalCredit      float64
	transactionCount int
}

type expenseSummaryCollector struct {
	byDept map[deptExpenseKey]*expenseDeptAgg
}

func newExpenseSummaryCollector() *expenseSummaryCollector {
	return &expenseSummaryCollector{byDept: make(map[deptExpenseKey]*expenseDeptAgg)}
}

func labelAfterCode(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, " - "); idx >= 0 {
		return strings.TrimSpace(raw[idx+3:])
	}
	return raw
}

func (c *expenseSummaryCollector) add(b Budget_Balance, ministryCol, deptCol string) {
	if b.segment1 == 0 && b.segment3 == 0 {
		return
	}
	k := deptExpenseKey{ministryID: b.segment3, deptCode: b.segment1}
	a, ok := c.byDept[k]
	if !ok {
		a = &expenseDeptAgg{
			ministryID:     b.segment3,
			ministryName:   labelAfterCode(ministryCol),
			departmentCode: b.segment1,
			departmentName: labelAfterCode(deptCol),
		}
		c.byDept[k] = a
	}
	if a.ministryName == "" && ministryCol != "" {
		a.ministryName = labelAfterCode(ministryCol)
	}
	if a.departmentName == "" && deptCol != "" {
		a.departmentName = labelAfterCode(deptCol)
	}
	a.totalDebit += b.debitamt
	a.totalCredit += b.creditamt
	a.transactionCount++
}

func (a *expenseDeptAgg) totalExpense() float64 {
	return a.totalDebit + a.totalCredit
}

func (c *expenseSummaryCollector) departmentCount() int {
	return len(c.byDept)
}

func (c *expenseSummaryCollector) ministryCount() int {
	seen := make(map[int]struct{}, len(c.byDept))
	for k := range c.byDept {
		seen[k.ministryID] = struct{}{}
	}
	return len(seen)
}

type orgBudgetInfo struct {
	budgetAmount sql.NullFloat64
	budgetName   string
	sayukName    string
}

func loadOrgBudgetByCode(db *sql.DB, fy int) (map[int]orgBudgetInfo, error) {
	rows, err := db.Query(
		`SELECT budget_code, budget_amount, budget_name, sayuk_name FROM public.org_budget WHERE fiscalyear = $1`,
		fy,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[int]orgBudgetInfo)
	for rows.Next() {
		var code int
		var info orgBudgetInfo
		var amount sql.NullFloat64
		if err := rows.Scan(&code, &amount, &info.budgetName, &info.sayukName); err != nil {
			return nil, err
		}
		info.budgetAmount = amount
		out[code] = info
	}
	return out, rows.Err()
}

func parseMinistryLedgerPeriod(rows [][]string) (start, end sql.NullTime, ok bool) {
	for i := 0; i < len(rows) && i < 5; i++ {
		if len(rows[i]) == 0 {
			continue
		}
		line := strings.TrimSpace(rows[i][0])
		if line == "" {
			continue
		}
		ps, pe, err := parsePeriodFromReportLine(line)
		if err != nil {
			continue
		}
		return sql.NullTime{Time: ps, Valid: true}, sql.NullTime{Time: pe, Valid: true}, true
	}
	return sql.NullTime{}, sql.NullTime{}, false
}

func insertOrgBalanceImportRun(tx *sql.Tx, fy int, sourceFile string, periodStart, periodEnd sql.NullTime) (int, error) {
	var runID int
	err := tx.QueryRow(
		`INSERT INTO public.org_balance_import_run (
			fiscalyear, period_start, period_end, source_filename
		) VALUES ($1, $2, $3, $4) RETURNING id`,
		fy,
		nullTime(periodStart),
		nullTime(periodEnd),
		sourceFile,
	).Scan(&runID)
	return runID, err
}

func nullTime(t sql.NullTime) interface{} {
	if !t.Valid {
		return nil
	}
	return t.Time
}

func nullFloat(v sql.NullFloat64) interface{} {
	if !v.Valid {
		return nil
	}
	return v.Float64
}

func insertExpenseSummaryRows(tx *sql.Tx, runID, fy int, collector *expenseSummaryCollector, budgets map[int]orgBudgetInfo) error {
	const qIns = `INSERT INTO public.org_expense_summary_row (
		run_id, fiscalyear, aggregation_level,
		ministry_id, ministry_name, department_code, department_name,
		total_debit, total_credit, total_expense, transaction_count,
		budget_amount, remaining_budget
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`

	type ministryRollup struct {
		ministryID   int
		ministryName string
		totalDebit   float64
		totalCredit  float64
		totalExpense float64
		txnCount     int
		budgetSum    float64
		hasBudget    bool
	}
	ministries := make(map[int]*ministryRollup)

	for _, a := range collector.byDept {
		expense := a.totalExpense()
		var budgetAmt sql.NullFloat64
		if b, ok := budgets[a.departmentCode]; ok && b.budgetAmount.Valid {
			budgetAmt = b.budgetAmount
		}
		ministryName := a.ministryName
		deptName := a.departmentName
		if b, ok := budgets[a.departmentCode]; ok {
			if deptName == "" {
				deptName = b.budgetName
			}
			if ministryName == "" {
				ministryName = b.sayukName
			}
		}
		var remaining sql.NullFloat64
		if budgetAmt.Valid {
			remaining = sql.NullFloat64{Float64: budgetAmt.Float64 - expense, Valid: true}
		}
		_, err := tx.Exec(qIns,
			runID, fy, orgExpenseAggDepartment,
			a.ministryID, nullString(ministryName), a.departmentCode, nullString(deptName),
			a.totalDebit, a.totalCredit, expense, a.transactionCount,
			nullFloat(budgetAmt), nullFloat(remaining),
		)
		if err != nil {
			return fmt.Errorf("insert department summary ministry=%d dept=%d: %w", a.ministryID, a.departmentCode, err)
		}

		m := ministries[a.ministryID]
		if m == nil {
			m = &ministryRollup{ministryID: a.ministryID, ministryName: ministryName}
			ministries[a.ministryID] = m
		}
		if m.ministryName == "" && ministryName != "" {
			m.ministryName = ministryName
		}
		m.totalDebit += a.totalDebit
		m.totalCredit += a.totalCredit
		m.totalExpense += expense
		m.txnCount += a.transactionCount
		if budgetAmt.Valid {
			m.budgetSum += budgetAmt.Float64
			m.hasBudget = true
		}
	}

	for _, m := range ministries {
		var budgetAmt sql.NullFloat64
		if m.hasBudget {
			budgetAmt = sql.NullFloat64{Float64: m.budgetSum, Valid: true}
		}
		var remaining sql.NullFloat64
		if budgetAmt.Valid {
			remaining = sql.NullFloat64{Float64: budgetAmt.Float64 - m.totalExpense, Valid: true}
		}
		_, err := tx.Exec(qIns,
			runID, fy, orgExpenseAggMinistry,
			m.ministryID, nullString(m.ministryName), nil, nil,
			m.totalDebit, m.totalCredit, m.totalExpense, m.txnCount,
			nullFloat(budgetAmt), nullFloat(remaining),
		)
		if err != nil {
			return fmt.Errorf("insert ministry summary ministry=%d: %w", m.ministryID, err)
		}
	}
	return nil
}

func nullString(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func tableExists(db *sql.DB, table string) (bool, error) {
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1`,
		table,
	).Scan(&n)
	return n > 0, err
}

func runMinistryOrgBalanceImport(db *sql.DB, ledgerPath string, fy int, quiet bool) error {
	rows, err := loadWorksheetRows(ledgerPath)
	if err != nil {
		return err
	}

	hasRun, err := tableExists(db, "org_balance_import_run")
	if err != nil {
		return fmt.Errorf("check summary tables: %w", err)
	}
	if !hasRun {
		return fmt.Errorf("missing public.org_balance_import_run — apply 2026/20260606120000_add_org_expense_summary/migration.sql first")
	}

	budgets, err := loadOrgBudgetByCode(db, fy)
	if err != nil {
		return fmt.Errorf("load org_budget: %w", err)
	}

	periodStart, periodEnd, _ := parseMinistryLedgerPeriod(rows)
	sourceFile := filepath.Base(ledgerPath)
	collector := newExpenseSummaryCollector()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	runID, err := insertOrgBalanceImportRun(tx, fy, sourceFile, periodStart, periodEnd)
	if err != nil {
		return fmt.Errorf("insert import run: %w", err)
	}

	if fy > 2020 {
		if _, err := tx.Exec(`DELETE FROM public.org_balance WHERE fiscalyear = $1`, fy); err != nil {
			return fmt.Errorf("delete org_balance: %w", err)
		}
	}

	nInsert, err := handleMinistryBalance(tx, rows, quiet, collector)
	if err != nil {
		return err
	}

	if err := insertExpenseSummaryRows(tx, runID, fy, collector, budgets); err != nil {
		return err
	}

	if _, err := tx.Exec(
		`UPDATE public.org_balance_import_run SET detail_row_count = $1 WHERE id = $2`,
		nInsert, runID,
	); err != nil {
		return fmt.Errorf("update import run: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("ministry import: committed run_id=%d org_balance rows=%d summary dept=%d ministry=%d file=%s",
		runID, nInsert, collector.departmentCount(), collector.ministryCount(), sourceFile)
	return nil
}

func previewMinistryOrgBalanceImport(db *sql.DB, ledgerPath string, fy int, quiet bool) (nInsert, deptSummary, ministrySummary int, err error) {
	_ = db
	_ = fy
	rows, err := loadWorksheetRows(ledgerPath)
	if err != nil {
		return 0, 0, 0, err
	}
	collector := newExpenseSummaryCollector()
	nInsert, err = handleMinistryBalance(nil, rows, quiet, collector)
	if err != nil {
		return 0, 0, 0, err
	}
	return nInsert, collector.departmentCount(), collector.ministryCount(), nil
}
