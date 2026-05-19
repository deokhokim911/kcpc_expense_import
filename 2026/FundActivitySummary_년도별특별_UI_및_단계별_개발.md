# 년도별 특별(Special) — Fund Activity Summary · Detail UI 및 단계별 개발 방향

**작성 목적**: 「연도만 선택」 상태에서 **특별 사역원(Special)** 을 선택했을 때, **Summary 엑셀에서 적재한 테이블**을 원본과 동일한 포맷으로 보여주고, **Income / Expenses** 를 링크로 두어 클릭 시 **하단에 General Ledger(Detail) 내역**을 표시하는 흐름을 제품·DB 관점으로 고정하고, 구현을 단계로 나눈다.

**관련 원본 파일**

| 구분 | 파일 예시 | 역할 |
|------|-----------|------|
| Summary | `FundActivitySummary.*.xlsx` — 시트 `Fund Activity Summary` | 펀드별 집계 한 줄(Income, Expenses 등) |
| Detail | `GeneralLedger.*.Special_web_posting.xlsx` — 시트 `General Ledger` | 거래 라인(Fund 열로 펀드 매칭) |

**선행 문서**: `FundActivitySummary_년도별특별_저장소_개발방향.md`(저장소 선택 근거), `FundActivitySummary_개발_분석_및_설계.md`(금액 정의·QB 매핑 이슈).

---

## 1. 사용자 플로우(결정)

1. 사용자가 **회계연도(또는 조회 기준 연도)** 를 선택한 뒤, 사역원 목록에서 **특별(Special) 계열 사역원**을 선택한다.  
   - 기존 앱의 「서점 특별 / 카페 특별」 등과 동일하게 **문자열·키 기반 판별**을 유지하되, 본 기능은 **해당 연도에 매핑된 Fund Activity 데이터가 있을 때** Summary 영역을 노출한다.

2. 화면 상단(또는 기존 요약 카드 대체 영역)에 **Fund Activity Summary 테이블**을 표시한다.  
   - **열 순서·헤더 라벨**은 Summary 엑셀과 동일:  
     `Fund`, `Beginning Balance`, `Income`, `Expenses`, `Net Income (Expense)`, `Transfer`, `Net Increase (Decrease)`, `Ending Balance`, `[Beginning of Fiscal Year] Balance`, 및 필요 시 상단 기간 문구(`period_start` ~ `period_end`).

3. 각 행에서 **Income**, **Expenses** 셀은 단순 텍스트가 아니라 **링크(또는 버튼형 링크)** 로 표시한다.  
   - 포커스·키보드 진입·스크린 리더용 레이블(`aria-label`)을 붙인다.

4. 사용자가 **Income** 또는 **Expenses** 링크를 클릭하면, 같은 페이지 **하단 영역**에 해당 **펀드(Fund) + 동일 보고 기간 + 클릭한 유형(Income vs Expenses)** 에 맞는 **Detail 목록**을 표시한다.  
   - Detail은 적재 시 설정된 **`income_expense_kind`** 로 필터해 보여준다(아래 §4 참고).

5. 다른 펀드 행을 선택하거나 다른 링크를 누르면 하단 Detail이 갱신된다.  
   - 선택 상태를 URL 쿼리나 로컬 상태로 유지할지는 구현 단계에서 결정한다.

---

## 2. 저장소 결정: 기존 테이블 vs 별도 테이블

본 기능은 **엑셀에서 내려온 Summary 숫자와 Detail 라인을 그대로 재현**하는 것이 목표이므로, **`org_budget` / `org_balance`에 끼워 넣지 않고** 아래와 같이 **별도 테이블**로 관리한다.

| 테이블 역할 | 설명 |
|-------------|------|
| **한 번의 파일 업로드·적재 단위** | Summary 파일과 Detail 파일을 각각 또는 묶음으로 **같은 `import_run`(가칭)** 에 묶어, 기간·연도·출처 파일명을 공유한다. |
| **Summary 라인** | 펀드당 1행 + Total 행 — 엑셀 컬럼과 1:1 매핑. |
| **Detail 라인** | G/L 거래 한 줄씩 — `Fund`, `Date`, `Debit`, `Credit`, 메모 등. 계정 구간 헤더만 있는 행은 표시 정책에 따라 제외하거나 `account_name` 파생 컬럼으로만 사용한다. |

이렇게 하면 운영에서 **QB 추출본이 단일 진실 원본**이 되고, 앱은 **조회·표시·드릴다운**에 집중할 수 있다.

---

## 3. 데이터 모델 초안(가칭)

구현 시 Prisma 모델명은 팀 규칙에 맞게 조정한다.

### 3.1 Import 실행 단위

- `fund_activity_import_run` — Prisma 모델명 동일, 테이블명 `fund_activity_import_run`  
  - `id`, `fiscalyear`(표시·검색용), `period_start`, `period_end`, `summary_imported_at`, `detail_imported_at`, `summary_source_filename`, `detail_source_filename`, `notes`, `created_at`  
  - 한 연도·한 기간 조합에 대해 **최신 run만 쓸지**, 목록에서 선택할지는 Phase 2 이후 옵션으로 둔다.

### 3.2 Summary 테이블

- `fund_activity_summary_row` — 테이블명 동일  
  - `run_id` → `fund_activity_import_run.id` (FK, 삭제 시 CASCADE)  
  - `fund_name`(문자열, 엑셀 `Fund`와 동일)  
  - `beginning_balance`, `income`, `expenses`, `net_income_expense`, `transfer`, `net_increase_decrease`, `ending_balance`, `beginning_of_fiscal_year_balance` — `DECIMAL(20,6)`  
  - `is_total_row`(boolean; `Total` 행 구분)

### 3.3 Detail 테이블

- `fund_activity_detail_line` — 테이블명 동일  
  - `run_id` → `fund_activity_import_run.id` (FK, 삭제 시 CASCADE; Summary와 **동일 run**·동일 보고 기간의 G/L이어야 함)  
  - `fund_name` — G/L `Fund` 열  
  - `income_expense_kind` — `Income` / `Expense` (Name 열 `Number - Text` 계정 구간 파싱, sticky 적용)  
  - `account_code` — `Number - Text`에서 추출한 계정 번호  
  - `line_date`, `transaction_number`, `transaction_type`(엑셀 `Type`, QB 유형), `contact`, `memo`, `reference_number`, `note`  
  - `debit`, `credit`, `amount`, `balance` — `DECIMAL(20,6)`  
  - `account_section` — 매칭된 `Number - Text` 헤더 전체 문자열  
  - `row_label` — 엑셀 `Name` 열 원문(거래 행·Beginning Balance 등 구분용, 선택)  
  - `source_row_order` — 원본 행 순서 보존용 정수(선택, 외부 파서에서 채움)

파싱 규칙: [`FundActivityDetail_TransactionType_파싱_개발방안.md`](FundActivityDetail_TransactionType_파싱_개발방안.md)

**키 매칭**: Summary 행과 Detail 행은 **`run_id` + `fund_name` 문자열 정규화(트림)** 로 연결한다. 펀드명 철자가 Summary와 G/L에서 완전히 같아야 한다.

---

## 4. Income / Expenses 링크와 Detail 필터 규칙

Summary의 **Income**, **Expenses** 는 QB Fund Activity 집계값이다. Detail은 G/L 적재 시 Name 열 `Number - Text` 계정 구간으로 분류한 **`income_expense_kind`** 를 사용한다 (적재 파서: `onfinance` `parseGeneralLedgerDetailLines`).

- **Income 링크**: 동일 `run_id`, 동일 `fund_name`, `income_expense_kind = 'Income'`.  
- **Expenses 링크**: 동일 `run_id`, 동일 `fund_name`, `income_expense_kind = 'Expense'`.

예시 SQL:

```sql
SELECT * FROM public.fund_activity_detail_line
WHERE run_id = $1 AND fund_name = $2 AND income_expense_kind = 'Income';
```

(이전 초안의 `Credit > 0` / `Debit > 0` 휴리스틱은 **사용하지 않음** — 계정 구간 기준이 1차 필터.)

Summary 합계와 Detail 라인 합이 **소수·반올림·분류 구간** 때문에 완전 일치하지 않을 수 있음을 UI에 작은 문구로 안내할 수 있다.

---

## 5. UI 구성 요약

| 영역 | 내용 |
|------|------|
| Summary 테이블 | 반응형 테이블 또는 가로 스크롤; 숫자는 통화 포맷; Total 행 강조 |
| Income / Expenses | `<button>` 또는 `<a>` 스타일 링크, `handleClick`으로 `selectedFund`, `drilldownKind: 'income' \| 'expenses'` 설정 |
| Detail 패널 | 테이블 또는 카드 리스트; 로딩·빈 상태·에러 메시지; 선택된 펀드·유형을 제목에 표시 |

접근성: 링크에 `aria-label` 예: `"TR-Vision 123 Contributions Income 상세 내역 보기"`.

---

## 6. 단계별 개발 로드맵

### Phase 1 — DB 스키마만 먼저 (적재는 별도 파서)

**범위**

1. **Prisma 마이그레이션**으로 `fund_activity_import_run`, `fund_activity_summary_row` 테이블을 **대상 DB에 생성**한다. 금액 컬럼은 `DECIMAL(20,6)` 로 통일한다.  
2. **Summary 엑셀 → DB 적재(파싱·INSERT)** 는 이 단계에 포함하지 않는다. **별도 파서/스크립트·배치**에서 `fund_activity_import_run` 1건 생성 후 `fund_activity_summary_row` 를 채우는 방식으로 진행한다.  
3. 앱 내 Summary xlsx 업로드 API는 Phase 1 후속 또는 Phase 2와 묶어 구현해도 된다.

**완료 기준**

- 마이그레이션 적용 후 DB에 위 두 테이블이 존재하고, FK·인덱스가 의도대로 동작한다.  
- (선택) Prisma Client로 빈 `import_run` + 샘플 `summary_row` 를 넣는 스크립트로 스키마 검증.

**마이그레이션 적용 참고**

- 마이그레이션 파일:  
  - Phase 1 — `prisma/migrations/20260507180000_add_fund_activity_summary_tables/migration.sql`  
  - Phase 2 — `prisma/migrations/20260508120000_add_fund_activity_detail_line/migration.sql`  
  일반적으로 `npx prisma migrate deploy` 로 순차 적용한다.  
- **기존 DB에 이미 다른 테이블이 있고 Prisma 이력(`_prisma_migrations`)이 비어 있는 경우** `migrate deploy` 가 P3005를 낼 수 있다. 그때는 Prisma 문서의 **baseline** 절차를 따르거나, 해당 SQL만 `prisma db execute --file …` 로 실행한 뒤 `prisma migrate resolve --applied <마이그레이션_폴더명>` 로 이력만 맞춘다.

**조회-only 후속(Phase 1 보완)**

- `GET` API로 `run_id` 또는 `fiscalyear` 기준 Summary 행 목록을 JSON으로 반환.  
- 클라이언트는 데이터가 있을 때만 테이블 렌더링(드릴다운 없음).

### Phase 2 — Detail 테이블 스키마 + 동일 run 조회 (적재는 별도 프로세서)

**범위**

1. **Prisma 마이그레이션**으로 `fund_activity_detail_line` 테이블을 생성한다. (`fund_activity_import_run` 에 `detail_lines` 관계 추가.) 금액 컬럼은 Phase 1과 동일하게 `DECIMAL(20,6)` 이다.  
2. **G/L 엑셀 → DB 적재(파싱·INSERT)** 는 이 단계에 포함하지 않는다. **별도 프로세서·파서·배치**에서 동일 `run_id` 로 `fund_activity_detail_line` 행을 넣는다. 적재 후 운영에서는 `fund_activity_import_run.detail_imported_at`, `detail_source_filename` 을 채우는 것을 권장한다.  
3. 앱 내 G/L 업로드 API·관리 UI는 Phase 2 후속 또는 Phase 3과 묶어도 된다.

**조회 후속(Phase 2 보완)**

- API: `GET …?runId=&fundName=&kind=income|expenses` 형태로 **기 적재된** Detail 목록을 반환(`income_expense_kind` 필터는 §4).  
- **동일 `run_id` 규칙**: Summary(`fund_activity_summary_row`)와 Detail은 같은 import run에 매달려야 드릴다운이 성립한다.

**완료 기준**

- 마이그레이션 적용 후 `fund_activity_detail_line` 이 존재하고, `run_id` FK·인덱스가 의도대로 동작한다.  
- (선택) 외부 프로세서로 샘플 G/L을 적재한 뒤, 파일 라인 수·펀드별 건수가 샘플과 일치하는지 검증한다.

### Phase 3 — `/Expense` 연도별 특별 플로우와 드릴다운 UI

- 연도 선택 후 **특별 사역원 선택 시** 기존 Sales/COGS/Net 전용 화면과 **분기**:  
  - 설정된 연도에 해당하는 **최신(또는 지정) `import_run`** 이 있으면 Fund Activity Summary 블록 표시.  
  - 없으면 안내 메시지(“해당 연도 Summary 데이터가 없습니다”) 및 업로드 안내(권한 있는 경우).  
- Summary 테이블에서 Income/Expenses 링크 클릭 시 **하단 Detail 패널** 표시.  
- **완료 기준**: 사용자 시나리오(§1)가 끝까지 동작한다.

### Phase 4 — 운영·품질

- Import 이력 목록, 파일명·기간 표시, 이전 run으로 보기(선택).  
- 펀드명 불일치 검사(Summary에만 있거나 G/L에만 있는 Fund 리포트).  
- 성능: Detail 행 수가 많을 때 페이지네이션 또는 가상 스크롤.  
- 보안: 업로드 API는 인가된 역할만 접근.

---

## 7. 범위에서 제외하거나 후순위로 둘 항목

- Summary 숫자를 **`org_balance`로 재계산**해 검증하는 배치(추후).  
- Transfer·Equity 전용 규칙 자동화(기존 설계 문서 §3.2~3.3).  
- 다국어 헤더.

---

## 8. 산출물 체크리스트

- [x] DB 마이그레이션: `fund_activity_import_run`, `fund_activity_summary_row` (Phase 1 스키마).  
- [x] DB 마이그레이션: `fund_activity_detail_line` (Phase 2 스키마).  
- [ ] Summary·Detail 파서(xlsx) — **별도 리포지토리/프로세서**에서 적재.  
- [ ] API: run 목록, Summary 조회, Detail 조회.  
- [ ] UI: 특별 사역원 분기 + Summary + 드릴다운 패널.  
- [ ] 운영 문서: 업로드 절차, 파일 명명 규칙, 기간 일치 요구사항.

---

*본 문서는 `GeneralLedger.20260501.Special_web_posting.xlsx`가 Detail 원천임을 전제로 하며, Summary 파일과 **동일 보고 기간**으로 pair 되어야 한다.*
