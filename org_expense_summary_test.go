package main

import (
	"testing"
)

func TestExpenseSummaryCollector_AddAndRollup(t *testing.T) {
	t.Parallel()
	c := newExpenseSummaryCollector()
	c.add(Budget_Balance{
		segment1: 255, segment3: 107,
		debitamt: 100, creditamt: -10,
	}, "107 - CHILDREN", "255 - MATERIALS")
	c.add(Budget_Balance{
		segment1: 255, segment3: 107,
		debitamt: 50, creditamt: 0,
	}, "107 - CHILDREN", "255 - MATERIALS")
	c.add(Budget_Balance{
		segment1: 210, segment3: 107,
		debitamt: 200, creditamt: -5,
	}, "107 - CHILDREN", "210 - NURSERY")

	if c.departmentCount() != 2 {
		t.Fatalf("dept count=%d want 2", c.departmentCount())
	}
	if c.ministryCount() != 1 {
		t.Fatalf("ministry count=%d want 1", c.ministryCount())
	}

	k := deptExpenseKey{107, 255}
	a := c.byDept[k]
	if a == nil {
		t.Fatal("missing dept agg")
	}
	if got := a.totalExpense(); got != 140 {
		t.Fatalf("totalExpense=%v want 140", got)
	}
	if a.transactionCount != 2 {
		t.Fatalf("txn count=%d want 2", a.transactionCount)
	}
}

func TestLabelAfterCode(t *testing.T) {
	t.Parallel()
	if got := labelAfterCode("255 - 교재비 / CLASS MATERIALS"); got != "교재비 / CLASS MATERIALS" {
		t.Fatalf("got %q", got)
	}
}

func TestParseMinistryLedgerPeriod(t *testing.T) {
	t.Parallel()
	rows := [][]string{
		{"KCPC"},
		{"Period of 07/01/2025 to 05/31/2026"},
	}
	start, end, ok := parseMinistryLedgerPeriod(rows)
	if !ok {
		t.Fatal("expected period")
	}
	if !start.Valid || !end.Valid {
		t.Fatal("null period")
	}
	if start.Time.Format("2006-01-02") != "2025-07-01" {
		t.Fatalf("start=%s", start.Time.Format("2006-01-02"))
	}
	if end.Time.Format("2006-01-02") != "2026-05-31" {
		t.Fatalf("end=%s", end.Time.Format("2006-01-02"))
	}
}
