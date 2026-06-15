package main

import (
	"testing"
)

func TestIsNumberText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"5180 - Payroll", true},
		{"4300 - Contributions", true},
		{"1865.1 - Contractors", true},
		{"1865.5 - Administrative Expenses", true},
		{" 5180 - X ", true},
		{"Random Header", false},
		{"5180", false},
		{"abc - Text", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isNumberText(tc.in); got != tc.want {
			t.Errorf("isNumberText(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseNumberText(t *testing.T) {
	t.Parallel()
	code, ok := parseNumberText("5180 - Payroll Expense")
	if !ok || code != "5180" {
		t.Fatalf("parseNumberText: got %q %v", code, ok)
	}
}

func TestClassifyAccountCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code string
		want string
	}{
		{"1865", incomeExpenseKindExpense},
		{"1865.1", incomeExpenseKindExpense},
		{"1865.5", incomeExpenseKindExpense},
		{"18650", incomeExpenseKindExpense},
		{"5120", incomeExpenseKindExpense},
		{"5180", incomeExpenseKindExpense},
		{"5179", incomeExpenseKindExpense},
		{"5119", ""},
		{"9010", incomeExpenseKindExpense},
		{"4300", incomeExpenseKindIncome},
		{"4510", incomeExpenseKindIncome},
		{"4600", ""},
		{"9011", ""},
	}
	for _, tc := range cases {
		if got := classifyAccountCode(tc.code); got != tc.want {
			t.Errorf("classifyAccountCode(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestParseGeneralLedgerDetailLines_StickyKind(t *testing.T) {
	t.Parallel()
	header := []string{"Name", "Date", "", "Type", "", "", "", "", "Fund", "", "", "", "Debit", "Credit", "Amount", "Balance"}
	rows := [][]string{
		header,
		{"", "01/01/2026", "", "Check", "", "", "", "", "Fund Early", "", "", "", "", "1", "1", ""},
		{"4300 - Income Accounts", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "01/15/2026", "1", "Check", "", "", "", "", "Fund A", "", "", "", "0", "100", "100", ""},
		{"5180 - Expense Accounts", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "01/16/2026", "2", "Bill", "", "", "", "", "Fund B", "", "", "", "50", "0", "50", ""},
		{"Not Number Text", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "01/17/2026", "3", "Check", "", "", "", "", "Fund C", "", "", "", "0", "10", "10", ""},
	}
	lines, err := parseGeneralLedgerDetailLines(rows, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (Fund Early skipped; Fund C sticky Expense)", len(lines))
	}
	if lines[0].fundName != "Fund A" || lines[0].incomeExpenseKind != incomeExpenseKindIncome {
		t.Fatalf("line0: fund=%q kind=%q", lines[0].fundName, lines[0].incomeExpenseKind)
	}
	if lines[0].accountCode.String != "4300" {
		t.Fatalf("line0 account_code=%q", lines[0].accountCode.String)
	}
	if lines[1].fundName != "Fund B" || lines[1].incomeExpenseKind != incomeExpenseKindExpense {
		t.Fatalf("line1: fund=%q kind=%q", lines[1].fundName, lines[1].incomeExpenseKind)
	}
	if lines[1].accountSection.String != "5180 - Expense Accounts" {
		t.Fatalf("line1 account_section=%q", lines[1].accountSection.String)
	}
	if lines[2].fundName != "Fund C" || lines[2].incomeExpenseKind != incomeExpenseKindExpense {
		t.Fatalf("line2: fund=%q kind=%q (sticky after non-Number-Text header)", lines[2].fundName, lines[2].incomeExpenseKind)
	}
}

func TestParseGeneralLedgerDetailLines_1865Subaccount(t *testing.T) {
	t.Parallel()
	header := []string{"Name", "Date", "", "Type", "", "", "", "", "Fund", "", "", "", "Debit", "Credit", "Amount", "Balance"}
	rows := [][]string{
		header,
		{"1865.1 - Contractors", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "01/15/2026", "", "Bill", "", "", "", "", "TR-Vision 123 Contributions", "", "", "", "100", "0", "100", ""},
	}
	lines, err := parseGeneralLedgerDetailLines(rows, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	if lines[0].incomeExpenseKind != incomeExpenseKindExpense {
		t.Fatalf("kind=%q", lines[0].incomeExpenseKind)
	}
	if lines[0].accountCode.String != "1865.1" {
		t.Fatalf("account_code=%q", lines[0].accountCode.String)
	}
}

func TestParseGeneralLedgerDetailLines_5120PensionExpense(t *testing.T) {
	t.Parallel()
	header := []string{"Name", "Date", "", "Type", "", "", "", "", "Fund", "", "", "", "Debit", "Credit", "Amount", "Balance"}
	rows := [][]string{
		header,
		{"4510 - Vision 123 Contributions", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "01/15/2026", "", "Check", "", "", "", "", "TR-Vision 123", "", "", "", "", "100", "100", ""},
		{"5120 - Pension", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "01/16/2026", "", "Check", "", "", "", "", "DF-Staff Pension", "", "", "", "", "50", "50", ""},
	}
	lines, err := parseGeneralLedgerDetailLines(rows, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[1].fundName != "DF-Staff Pension" || lines[1].incomeExpenseKind != incomeExpenseKindExpense {
		t.Fatalf("pension line: fund=%q kind=%q", lines[1].fundName, lines[1].incomeExpenseKind)
	}
	if lines[1].accountCode.String != "5120" {
		t.Fatalf("account_code=%q", lines[1].accountCode.String)
	}
}

func TestParseGeneralLedgerDetailLines_OutOfRangeHeaderKeepsKind(t *testing.T) {
	t.Parallel()
	header := []string{"Name", "Date", "", "Type", "", "", "", "", "Fund", "", "", "", "Debit", "Credit", "Amount", "Balance"}
	rows := [][]string{
		header,
		{"4300 - Income", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "01/15/2026", "", "Check", "", "", "", "", "Fund A", "", "", "", "", "10", "10", ""},
		{"4600 - Other", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "01/16/2026", "", "Check", "", "", "", "", "Fund A", "", "", "", "", "20", "20", ""},
	}
	lines, err := parseGeneralLedgerDetailLines(rows, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	for _, ln := range lines {
		if ln.incomeExpenseKind != incomeExpenseKindIncome {
			t.Fatalf("expected sticky Income, got %q", ln.incomeExpenseKind)
		}
	}
}
