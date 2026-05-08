package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	_ "github.com/lib/pq"
)

var targetCodes = []int{4870, 4880, 5001, 5002}

var GeneralLedgerFileName = "./2026/GeneralLedger.20260501_web_posting.xlsx"

func isValidRangeCode(code int) bool {

	// iterate using the for loop
	for i := 0; i < len(targetCodes); i++ {
		// check
		if targetCodes[i] == code {
			// return true
			return true
		}
	}
	return false
}

const fiscalYear = 2026

// ledgerRowMinCols pads short Excel rows so rec[0]..rec[14] are safe for ministry ledger parsing.
const ledgerRowMinCols = 16

// DB 연결은 환경변수로만 받는다 (비밀번호/호스트를 레포에 커밋하지 않기 위함).
// 사용 가능한 환경변수:
// - DATABASE_URL: postgres://USER:PASSWORD@HOST:PORT/DB?sslmode=disable
// - 또는 PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE 조합

func buildPostgresConnStringFromEnv() (string, error) {
	if url := strings.TrimSpace(os.Getenv("DATABASE_URL")); url != "" {
		return url, nil
	}

	host := strings.TrimSpace(os.Getenv("PGHOST"))
	port := strings.TrimSpace(os.Getenv("PGPORT"))
	user := strings.TrimSpace(os.Getenv("PGUSER"))
	password := os.Getenv("PGPASSWORD")
	dbname := strings.TrimSpace(os.Getenv("PGDATABASE"))

	var missing []string
	if host == "" {
		missing = append(missing, "PGHOST")
	}
	if port == "" {
		missing = append(missing, "PGPORT")
	}
	if user == "" {
		missing = append(missing, "PGUSER")
	}
	if password == "" {
		missing = append(missing, "PGPASSWORD")
	}
	if dbname == "" {
		missing = append(missing, "PGDATABASE")
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing DB env vars: %s (or set DATABASE_URL)", strings.Join(missing, ", "))
	}

	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname), nil
}

type Budget_Balance struct {
	fiscalyear  int
	budget_cd   int
	create_dt   time.Time
	segment1    int
	segment2    int
	segment3    int
	account_id  string
	accountdesc string
	reference   string
	jrnl_cd     string
	transdesc   string
	debitamt    float64
	creditamt   float64
	balance     float64
	job_id      string
	transamt    float64
	total_cost  float64
}

func isValidCode(id, fiscalYear int, db *sql.DB) error {
	var codeDept int
	var budget_name string
	err := db.QueryRow("SELECT budget_code, budget_name FROM public.org_budget WHERE budget_code = $1 AND fiscalyear = $2", id, fiscalYear).Scan(&codeDept, &budget_name)
	if err != nil {
		return err
	}
	log.Printf("Retrieved budget_cd : %d, budget_name : %s ", codeDept, budget_name)
	return nil
}

func updateBudgetBalance(update Budget_Balance, db *sql.DB) (int, error) {
	sqlStatment := "INSERT INTO public.org_balance (fiscalyear, budget_cd, create_dt, segment1, segment2, segment3, account_id, accountdesc, reference, jrnl_cd, transdesc, debitamt, creditamt, total_cost, balance, job_id, transamt) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17) RETURNING orgbal_id;"

	var orabal_id int
	err := db.QueryRow(sqlStatment,
		update.fiscalyear,
		update.budget_cd,
		update.create_dt,
		update.segment1,
		update.segment2,
		update.segment3,
		update.account_id,
		update.accountdesc,
		update.reference,
		update.jrnl_cd,
		update.transdesc,
		update.debitamt,
		update.creditamt,
		update.total_cost,
		update.balance,
		update.job_id,
		update.transamt).Scan(&orabal_id)
	if err != nil {
		return -1, err
	}
	return orabal_id, nil
}

func getBudgetBalance(fiscalYear, budget_cd int, record []string) (Budget_Balance, error) {
	var balance Budget_Balance
	balance.fiscalyear = fiscalYear
	balance.budget_cd = budget_cd

	if len(record[0]) > 1 {
		balance.account_id = record[0]
		accounts := strings.Split(record[0], "-")
		balance.segment1, _ = strconv.Atoi(accounts[0])
		balance.segment2, _ = strconv.Atoi(accounts[1])
		balance.segment3, _ = strconv.Atoi(accounts[2])
	}

	if len(record[1]) > 1 {
		balance.accountdesc = record[1]
	} else {
		balance.accountdesc = ""
	}

	if len(record[2]) > 1 {
		// 문자열 형식과 일치하는 레이아웃 지정
		layout := "1/2/06"
		createdAt, err := time.Parse(layout, record[2])
		if err != nil {
			log.Fatalf("Error parsing date: %q", err)
		} else {
			balance.create_dt = createdAt
		}
		log.Printf("org [%v] -> [%v]]", record[2], createdAt)
	}

	if len(record[3]) > 1 {
		balance.reference = record[3]
	} else {
		balance.reference = ""
	}

	if len(record[4]) > 1 {
		balance.jrnl_cd = record[4]
	} else {
		balance.jrnl_cd = ""
	}

	if len(record[5]) > 1 {
		balance.transdesc = record[5]
	} else {
		balance.transdesc = ""
	}

	if len(record[6]) > 1 {
		clean6 := strings.Replace(record[6], ",", "", -1)
		balance.debitamt, _ = strconv.ParseFloat(clean6, 64)
	} else {
		balance.debitamt = 0.0
	}

	if len(record[7]) > 1 {
		clean7 := strings.Replace(record[7], ",", "", -1)
		balance.creditamt, _ = strconv.ParseFloat(clean7, 64)
	} else {
		balance.creditamt = 0.0
	}

	if len(record[8]) > 1 {
		clean8 := strings.Replace(record[8], ",", "", -1)
		balance.balance, _ = strconv.ParseFloat(clean8, 64)
	} else {
		balance.balance = 0.0
	}

	if len(record[9]) > 1 {
		balance.job_id = record[9]
	} else {
		balance.job_id = ""
	}

	if len(record[10]) > 1 {
		clean10 := strings.Replace(record[10], ",", "", -1)
		balance.transamt, _ = strconv.ParseFloat(clean10, 64)
	} else {
		balance.transamt = 0.0
	}
	// 정산을 위해 데빗과 크래딧 비용을 하나의 컬럼에 넣어 준다
	balance.total_cost = balance.debitamt + balance.creditamt

	return balance, nil
}

func getAfterCharacter(s, sep string) string {
	parts := strings.Split(s, sep)
	if len(parts) > 1 {
		return parts[1] // 특정 문자 이후의 문자열 반환
	}
	return "" // 특정 문자가 없으면 빈 문자열 반환
}

func removeQuote(s string) string {
		returnStr := strings.ReplaceAll(s, `"`, "")
		return strings.ReplaceAll(strings.ReplaceAll(returnStr, ",", ""), " ", "")
}

func padRow(rec []string, minCols int) []string {
	if len(rec) >= minCols {
		return rec
	}
	out := make([]string, minCols)
	copy(out, rec)
	return out
}

func loadWorksheetRows(path string) ([][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	if sheet == "" {
		return nil, fmt.Errorf("no worksheets in %s", path)
	}
	return f.GetRows(sheet)
}

func handleMinistryBalance(db *sql.DB, rows [][]string, quiet bool) int {
	var account_cd int
	var accountName string

	var balance_budget, previous_balance Budget_Balance
	processed := 0

	for _, raw := range rows {
		rec := padRow(raw, ledgerRowMinCols)
		balance_budget = Budget_Balance{}
		balance_budget.fiscalyear = fiscalYear

		if len(rec[0]) > 1 {

			if !strings.Contains(rec[0], " - ") {
				continue
			}

			accountId := strings.Split(rec[0], " - ")[0]
			accountName = getAfterCharacter(rec[0], " - ")

			// balance_budget.account_id = accountId

			if len(accountId) > 4 || len(accountId) < 1 {
				continue
			}

			if strings.Contains(accountId, "Totalfor") {
				account_cd = 0
				previous_balance = Budget_Balance{}
				continue
			} 

			noAccountId, err := strconv.Atoi(accountId)
			if err != nil {
				log.Println("Error : ", rec[0], err.Error())
				continue
			} else {
				account_cd = noAccountId
			}
		} 
		
		if account_cd > 0 {

			if len(rec[13]) < 1 {
				continue
			}

			balance_budget.create_dt = previous_balance.create_dt

			balance_budget.budget_cd = account_cd
			balance_budget.segment2 = account_cd	
			balance_budget.accountdesc = accountName

			if len(rec[9]) > 3 {
				ministryId, _ := strconv.Atoi(strings.Trim(strings.Split(rec[9], " - ")[0], " "))
				// ministryName := strings.Trim(getAfterCharacter(rec[9], " - "), " ")	
				// fmt.Printf("****** ministryId = %v, ministryName = %v\n", ministryId, ministryName)		
				balance_budget.segment3 = ministryId
			}

			if len(rec[10]) > 3 {
				deptId, _ := strconv.Atoi(strings.Trim(strings.Split(rec[10], " - ")[0], " "))
				// deptName := strings.Trim(getAfterCharacter(rec[10], " - "), " ")	
				// fmt.Printf("****** deptId = %v, deptName = %v\n", deptId, deptName)			

				balance_budget.budget_cd = deptId
				balance_budget.segment1 = deptId
			}

			if len(rec[1]) > 1 {
				issuedAt := strings.ReplaceAll(rec[1], " ", "")
				layout := "01/02/2006" // MM/DD/YYYY 형식
				createdAt, err := time.Parse(layout, issuedAt)
				if err != nil {
					log.Printf("ministry: skip row, bad date %q: %v", issuedAt, err)
					continue
				}
				balance_budget.create_dt = createdAt
			}

			if len(rec[6]) > 1 {
				balance_budget.reference = rec[6]
			}

			if len(rec[7]) > 1 {
				balance_budget.transdesc = fmt.Sprintf("%s | %s", rec[4], rec[7])
			}

			if len(rec[3]) > 1 {
				jrnl_cd := strings.ReplaceAll(rec[3], " ", "")
				balance_budget.jrnl_cd = jrnl_cd
			}

			if len(rec[11]) > 1 {
				clean11 := removeQuote(rec[11])
				debitamt, _ := strconv.ParseFloat(clean11, 64)
				balance_budget.debitamt = debitamt
			}

			if len(rec[12]) > 1 {
				clean12 := removeQuote(rec[12])
				creditamt, _ := strconv.ParseFloat(clean12, 64)
				balance_budget.creditamt = creditamt * -1
			}

			if len(rec[13]) > 1 {
				clean13 := removeQuote(rec[13])
				transamt, _ := strconv.ParseFloat(clean13, 64)
				balance_budget.transamt = transamt
			}

			if len(rec[14]) > 1 {
				clean14 := removeQuote(rec[14])
				balance, _ := strconv.ParseFloat(clean14, 64)
				balance_budget.balance = balance
			}

			balance_budget.account_id = fmt.Sprintf("%v-%v-%v", balance_budget.segment1, balance_budget.segment2, balance_budget.segment3)

			if !quiet {
				fmt.Println("MinistryBalance >>> ", balance_budget)
			}
			processed++
			if db != nil {
				noRow, err := updateBudgetBalance(balance_budget, db)
				if err != nil {
					log.Println("Error : ", err.Error())
				} else {
					log.Println("Row Inserted : ", noRow)
				}
			}

			previous_balance = balance_budget
		}
	}
	return processed
}

func main() {
	dryRun := flag.Bool("dry-run", false, "parse only; no DB writes (use with -import-fund-summary or -import-org-balance)")
	quiet := flag.Bool("quiet", false, "suppress per-row printed ledger lines (legacy -import-org-balance only)")
	importFundSummary := flag.Bool("import-fund-summary", false, "insert fund_activity_import_run + fund_activity_summary_row from Summary xlsx (does not touch org_*)")
	summaryXlsx := flag.String("summary-xlsx", "", "path to FundActivitySummary *.xlsx")
	detailXlsx := flag.String("detail-xlsx", "", "optional General Ledger xlsx: period cross-check + detail_source_filename")
	fiscalYearFlag := flag.Int("fiscal-year", fiscalYear, "fiscal year stored on fund_activity_import_run")
	importFundDetail := flag.Bool("import-fund-detail", false, "insert fund_activity_detail_line from General Ledger Special xlsx (does not touch org_*)")
	detailLedgerXlsx := flag.String("detail-ledger-xlsx", "", "path to General Ledger *.xlsx (sheet General Ledger)")
	runIDFlag := flag.Int("run-id", 0, "fund_activity_import_run id for detail import (or use -match-run-by-period)")
	matchRunByPeriod := flag.Bool("match-run-by-period", false, "resolve run_id by period parsed from the ledger file vs fund_activity_import_run")
	replaceFundActivityYear := flag.Bool("replace-fund-activity-year", false, "delete ALL fund_activity_* for -fiscal-year, then import Summary + Detail in one transaction (does not touch org_*)")
	importOrgBalance := flag.Bool("import-org-balance", false, "ministry General Ledger -> org_balance (modifies org_*; Special ledger removed — use fund activity import instead)")
	ministryLedgerXlsx := flag.String("ministry-ledger-xlsx", GeneralLedgerFileName, "path to ministry General Ledger xlsx (first worksheet)")
	flag.Parse()

	if *replaceFundActivityYear {
		if *summaryXlsx == "" || *detailLedgerXlsx == "" {
			log.Fatal("-replace-fund-activity-year requires -summary-xlsx and -detail-ledger-xlsx")
		}
		psqlInfo, err := buildPostgresConnStringFromEnv()
		if err != nil {
			log.Fatal(err)
		}
		db, err := sql.Open("postgres", psqlInfo)
		if err != nil {
			log.Fatalf("open db: %v", err)
		}
		defer db.Close()
		if err := db.Ping(); err != nil {
			log.Fatalf("ping db: %v", err)
		}
		if err := runFundActivityFiscalYearReplace(db, *fiscalYearFlag, *summaryXlsx, *detailLedgerXlsx, *dryRun); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *importFundDetail {
		if *detailLedgerXlsx == "" {
			log.Fatal("-detail-ledger-xlsx is required with -import-fund-detail")
		}
		if *runIDFlag <= 0 && !*matchRunByPeriod {
			log.Fatal("set -run-id <id> or -match-run-by-period with -import-fund-detail")
		}
		needDB := !*dryRun || (*matchRunByPeriod && *runIDFlag <= 0)
		var db *sql.DB
		if needDB {
			psqlInfo, err := buildPostgresConnStringFromEnv()
			if err != nil {
				log.Fatal(err)
			}
			db, err = sql.Open("postgres", psqlInfo)
			if err != nil {
				log.Fatalf("open db: %v", err)
			}
			defer db.Close()
			if err := db.Ping(); err != nil {
				log.Fatalf("ping db: %v", err)
			}
		}
		if err := runFundActivityDetailImport(db, *detailLedgerXlsx, *runIDFlag, *matchRunByPeriod, *dryRun); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *importFundSummary {
		if *summaryXlsx == "" {
			log.Fatal("-summary-xlsx is required with -import-fund-summary")
		}
		if *dryRun {
			if err := runFundActivitySummaryImport(nil, *fiscalYearFlag, *summaryXlsx, *detailXlsx, true); err != nil {
				log.Fatal(err)
			}
			return
		}
		psqlInfo, err := buildPostgresConnStringFromEnv()
		if err != nil {
			log.Fatal(err)
		}
		db, err := sql.Open("postgres", psqlInfo)
		if err != nil {
			log.Fatalf("open db: %v", err)
		}
		defer db.Close()
		if err := db.Ping(); err != nil {
			log.Fatalf("ping db: %v", err)
		}
		if err := runFundActivitySummaryImport(db, *fiscalYearFlag, *summaryXlsx, *detailXlsx, false); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *dryRun {
		if !*importOrgBalance {
			printUsage()
			os.Exit(2)
		}
		runDryRun(*ministryLedgerXlsx, *quiet)
		return
	}

	if !*importOrgBalance {
		fmt.Println("No action: legacy org_balance import is disabled by default.")
		fmt.Println("Use -replace-fund-activity-year -fiscal-year N -summary-xlsx <s.xlsx> -detail-ledger-xlsx <gl.xlsx> for full fiscal reload (recommended).")
		fmt.Println("Or -import-fund-summary / -import-fund-detail separately.")
		fmt.Println("Or pass -import-org-balance only if you intend to modify org_balance.")
		os.Exit(0)
	}

	psqlInfo, err := buildPostgresConnStringFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatalf("Error opening database: %q", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("Error pinging database: %q", err)
	}
	fmt.Println("Successfully connected!")

	if fiscalYear > 2020 {
		sqlStatment := fmt.Sprintf("delete from public.org_balance where fiscalyear = %d;", fiscalYear)
		log.Println("sqlStatment : ", sqlStatment)

		_, err = db.Exec(sqlStatment)
		if err != nil {
			log.Fatalf("Failed to insert data: %v", err)
		}
		fmt.Println("Data Deleted successfully!, fiscal year = ", fiscalYear)
	}

	ministryRows, err := loadWorksheetRows(*ministryLedgerXlsx)
	if err != nil {
		log.Fatal(err)
	}
	handleMinistryBalance(db, ministryRows, *quiet)
}

func runDryRun(ministryLedgerPath string, quiet bool) {
	ministryRows, err := loadWorksheetRows(ministryLedgerPath)
	if err != nil {
		log.Fatalf("ministry ledger: %v", err)
	}
	nMin := handleMinistryBalance(nil, ministryRows, quiet)
	fmt.Printf("[dry-run] ministry ledger rows processed: %d\n", nMin)
}
