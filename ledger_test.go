package main

import (
	"os"
	"testing"
)

func TestPadRow(t *testing.T) {
	t.Parallel()
	r := padRow([]string{"a", "b"}, 5)
	if len(r) != 5 {
		t.Fatalf("len=%d want 5", len(r))
	}
	if r[0] != "a" || r[1] != "b" || r[2] != "" {
		t.Fatalf("got %#v", r)
	}
	if padRow([]string{"x"}, ledgerRowMinCols)[ledgerRowMinCols-1] != "" {
		t.Fatal("trailing cells should be empty")
	}
}

func TestLoadWorksheetRows_MinistryFile(t *testing.T) {
	if _, err := os.Stat(GeneralLedgerFileName); err != nil {
		t.Skip("skip: ", GeneralLedgerFileName, " not available:", err)
	}
	rows, err := loadWorksheetRows(GeneralLedgerFileName)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 10 {
		t.Fatalf("expected header + data rows, got %d", len(rows))
	}
}

// TestHandleMinistryBalanceDryRun exercises full ministry parse without DB (nil db).
func TestHandleMinistryBalanceDryRun(t *testing.T) {
	if _, err := os.Stat(GeneralLedgerFileName); err != nil {
		t.Skip("skip: ", GeneralLedgerFileName, " not available:", err)
	}
	rows, err := loadWorksheetRows(GeneralLedgerFileName)
	if err != nil {
		t.Fatal(err)
	}
	n := handleMinistryBalance(nil, rows, true)
	if n == 0 {
		t.Fatal("expected at least one processed ministry ledger row")
	}
	t.Logf("ministry rows processed: %d", n)
}
