## FY2026 사역원 예산(budget_amount) 업데이트 SQL 문서

### 생성 대상
- **원본 파일**: `2026/FY2026 Ministry List with budget - revised for rounding correction.xlsx`
- **추출 규칙**
  - 1열이 **숫자(예산코드)** 이고, 2열이 **항목명**, 3열이 **예산금액**인 행만 사용
  - `... 합계` 행과 사역원 헤더(예: `101 - ...`)는 제외

### 업데이트 대상 테이블/컬럼
이 레포의 코드(`main.go`) 기준으로는 아래 형태로 사용 중입니다.
- **테이블**: `public.org_budget`
- **키 컬럼(조건)**: `fiscalyear = 2026 AND budget_code = <code>`
- **수정 컬럼**: `budget_amount`

요청하신 테이블명이 **`org.budget`** 인 환경이라면, SQL 파일에서 테이블명을 다음처럼 바꿔서 실행하세요.
- `public.org_budget` → `org.budget`

### 생성된 SQL 파일
- `2026/FY2026_org_budget_amount_update.sql`

### 실행 예시 (psql)

```bash
psql "<YOUR_CONNECTION_STRING>" -f "2026/FY2026_org_budget_amount_update.sql"
```

### 실행 전 점검(권장)
업데이트가 실제로 매칭되는지 먼저 확인하세요.

```sql
SELECT budget_code, budget_amount
FROM public.org_budget
WHERE fiscalyear = 2026
  AND budget_code IN (10, 20, 25);
```

### 주의사항
- 이 스크립트는 **(fiscalyear, budget_code)** 로만 업데이트합니다.
  - 만약 `budget_code`가 연도 내에서 유일하지 않다면(중복 존재), 추가 조건(예: `sayuk_name`, `budget_name`)이 필요합니다.
