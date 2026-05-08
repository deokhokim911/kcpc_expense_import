# 사역원(일반) General Ledger → `org_balance` 적재 — Fund Activity 스크립트와의 정렬 계획

**작성 목적**: `main.go`에서 **Special(Special web posting) 원장은 더 이상 `org_balance`에 넣지 않고**, **사역원 엑셀(일반 `GeneralLedger.*_web_posting.xlsx` 스타일)**만 `handleMinistryBalance`로 처리하도록 정리한 뒤, **Fund Activity 쪽 `scripts/*.sh`와 같은 운영 방식**으로 사역원 적재를 쓰기 위한 단계·스크립트·플래그 계획을 정리한다.

**전제 (코드 기준, 2026-05 기준)**  
- **특별(Special) G/L** → `fund_activity_*` 테이블 + `import_fund_activity_*.sh` / `-replace-fund-activity-year`  
- **사역원(일반) G/L** → `public.org_balance` + `handleMinistryBalance` (`-import-org-balance`)

---

## 1. 현재 동작 (정리 후)

| 구분 | 입력 파일 성격 | 처리 함수 | 대상 테이블 |
|------|----------------|-----------|-------------|
| 사역원 | 일반 웹 포스팅 원장 (첫 번째 시트) | `handleMinistryBalance` | `org_balance` |
| 특별(Fund Activity) | Summary / Special G/L | 별도 패키지 로직 | `fund_activity_*` (Special은 여기로만) |

`handleSpecialBalace` 및 Special 원장을 통한 `org_balance` INSERT 경로는 **제거**되었다.

---

## 2. Fund Activity 스크립트 패턴 (참고 표준)

다음을 사역원 적재에도 그대로 맞춘다.

1. **단일 진입 셸 스크립트** (`scripts/*.sh`): 레포 루트에서 `go run .` + 고정 플래그 조합  
2. **환경변수로 경로·연도 오버라이드** (선택)  
3. **실행 전 stderr 안내** (무엇을 하는지, 모드 dry/live)  
4. **Go 쪽 진행 로그** (이미 Fund Activity는 건별/구간별 출력 적용됨)

사역원 경로는 **`main.go` 상단 상수 `GeneralLedgerFileName`** 을 기본값으로 두되, **CLI로 파일 경로를 바꿀 수 있게 하는 것**이 스크립트화의 핵심이다.

---

## 3. 이미 반영된 CLI 변경

- `-import-org-balance` 사용 시  
  **`-ministry-ledger-xlsx <path>`** 로 사역원 엑셀 경로 지정 가능 (기본: 기존 `GeneralLedgerFileName` 상수와 동일).  
- `-dry-run -import-org-balance` 도 동일 경로로 파싱만 수행.

예시:

```bash
go run . -import-org-balance -ministry-ledger-xlsx ./2026/GeneralLedger.20260501_web_posting.xlsx
go run . -dry-run -import-org-balance -ministry-ledger-xlsx ./2026/GeneralLedger.20260501_web_posting.xlsx -quiet
```

---

## 4. 권장 스크립트 초안 — `scripts/import_ministry_org_balance.sh`

**역할**: 사역원 원장만 읽어 `org_balance` 재적재 (실행 시 해당 회계연도 `org_balance` 삭제 후 삽입 — 기존 `main` 동작 유지).

**제안 내용**:

```bash
#!/usr/bin/env bash
# 환경변수 예: MINISTRY_LEDGER_XLSX (기본은 레포 내 GeneralLedger.*_web_posting.xlsx)
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
LEDGER="${MINISTRY_LEDGER_XLSX:-$ROOT/2026/GeneralLedger.20260501_web_posting.xlsx}"
exec go run . -import-org-balance -ministry-ledger-xlsx "$LEDGER" "$@"
```

**dry-run**:

```bash
go run . -dry-run -import-org-balance -ministry-ledger-xlsx ... -quiet
```

**Fund Activity 스크립트와 동일하게**:

- 실행 직후 `==> Ministry org_balance import` 같은 **헤더 출력**  
- `--dry-run` / `-n` 시 `-dry-run` 전달

※ 레포에 **`scripts/import_ministry_org_balance.sh`** 를 추가하였다. 경로·dry-run·`MINISTRY_IMPORT_QUIET` 는 스크립트 주석 참고.

---

## 5. 추가 개선 후보 (선택 로드맵)

| 항목 | 설명 |
|------|------|
| 진행 로그 | `handleMinistryBalance` 에서 N건마다 또는 매 행 로그 (Fund Activity detail 과 유사하게 튜닝) |
| `-fiscal-year` 연동 | 현재 `org_balance` 삭제·삽입 연도는 `main.go` 의 `fiscalYear` 상수. 향후 플래그로 빼면 스크립트에서 `FISCAL_YEAR` 완전 연동 가능 |
| 트랜잭션 | 삭제+삽입을 단일 DB 트랜잭션으로 묶어 실패 시 롤백 (운영 안전성) |
| 설정 분리 | DB 호스트·비밀번호를 환경변수로만 읽기 (`DATABASE_URL` 등) |

---

## 6. 운영 시 주의

- **Fund Activity** 적재는 **`org_*`를 건드리지 않음**.  
- **사역원 적재**는 **`org_balance` 대량 삭제·INSERT** 이므로, 실행 계정·대상 DB·회계연도를 반드시 확인할 것.  
- Special 원장이 필요하면 **`import_fund_activity_fiscal_year.sh`** 또는 동등한 Fund Activity 플래그만 사용.

---

*본 문서는 코드 변경(사역원만 `org_balance`, Special 경로 제거)과 스크립트 정렬 목적으로 작성하였다.*
