package main

import (
	"os"
	"testing"
)

func TestParseCurrentSpecialLedger_1865Sections(t *testing.T) {
	path := "current/GeneralLedger.Special.20260508.xlsx"
	if _, err := os.Stat(path); err != nil {
		t.Skip("skip:", path, err)
	}
	rows, err := loadGeneralLedgerRows(path)
	if err != nil {
		t.Fatal(err)
	}
	hdr := findGeneralLedgerHeaderRow(rows)
	if hdr < 0 {
		t.Fatal("no header")
	}
	lines, err := parseGeneralLedgerDetailLines(rows, hdr)
	if err != nil {
		t.Fatal(err)
	}
	var expense1865 int
	for _, ln := range lines {
		if ln.incomeExpenseKind == incomeExpenseKindExpense && ln.accountCode.Valid {
			if len(ln.accountCode.String) >= 4 && ln.accountCode.String[:4] == "1865" {
				expense1865++
			}
		}
	}
	if expense1865 == 0 {
		t.Fatalf("expected detail lines under 1865.* sections, got 0 (total lines=%d)", len(lines))
	}
	t.Logf("total detail lines=%d, 1865.* expense lines=%d", len(lines), expense1865)
}
