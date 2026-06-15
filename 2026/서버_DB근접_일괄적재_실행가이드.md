# 서버(DB 호스트)에서 일괄 적재 실행 가이드

**작성일**: 2026-05-30  
**목적**: 로컬 Mac/PC에서 Postgres에 원격 접속해 `run_all_imports` 를 돌리면 **네트워크 지연**으로 시간이 길어지는 문제를 줄이기 위해, **DB와 같은 서버(또는 동일 VPC·저지연 구간)** 에서 기존 스크립트를 **기능 변경 없이** 실행하는 방법을 정리한다.

**관련**

- 스크립트: [`scripts/run_all_imports.sh`](../scripts/run_all_imports.sh)
- env 샘플: [`scripts/run_all_imports.env_sample`](../scripts/run_all_imports.env_sample)
- 로컬 env 경로 규칙: `current/` + **파일명만** ([`load_run_all_imports_env.sh`](../scripts/lib/load_run_all_imports_env.sh))

---

## 1. 왜 서버에서 실행하는가

| 실행 위치 | DB 트래픽 경로 | 특징 |
|-----------|----------------|------|
| **로컬** | 노트북 → 인터넷/VPN → DB 서버 | `COUNT`·대량 `INSERT`·트랜잭션이 **왕복 지연**에 민감 → 느림 |
| **DB 서버(또는 동일 망)** | `localhost` / 사설 IP | 지연 최소, 동일 Go·동일 스크립트 |

적재 로직·미리보기(dry-run)·확인 프롬프트는 **변경하지 않는다**. 실행 위치와 `DATABASE_URL` 만 바꾼다.

```mermaid
flowchart LR
  subgraph slow [로컬 실행 - 네트워크 나쁠 때]
    L[노트북 Go] -->|많은 SQL 왕복| R[(원격 Postgres)]
  end
  subgraph fast [서버 실행 - 권장]
    S[서버 Go] -->|localhost| DB[(동일 호스트 Postgres)]
  end
```

---

## 2. 전제 조건

### 2.1 서버 측

| 항목 | 권장 |
|------|------|
| OS | Linux (Ubuntu 등) — DB가 돌아가는 VM/베어메탈 |
| Go | **1.22+** (`go version`) — 미설치 시 **§3** 참고 (`go.mod` 는 1.22.4 기준) |
| Git | 레포 clone·업데이트용 — `sudo apt install -y git` |
| 디스크 | `current/` 엑셀 3개 + 레포 수백 MB 여유 |
| DB 접근 | 서버 OS 사용자가 Postgres에 접속 가능 (`psql` 테스트) |

### 2.2 로컬 측 (준비만)

- 엑셀 3종 export (QB)
- 서버로 파일 전송 수단: `scp`, `rsync`, SFTP, 공유 폴더 등
- 서버 SSH 접속 계정

### 2.3 DB 마이그레이션

서버 DB에 아직 없다면 **한 번만** 적용 (로컬과 동일 SQL):

- [`2026/migration.sql`](migration.sql)
- [`2026/20260508120000_add_fund_activity_detail_line/migration.sql`](20260508120000_add_fund_activity_detail_line/migration.sql)
- [`2026/20260508130000_add_income_expense_kind/migration.sql`](20260508130000_add_income_expense_kind/migration.sql)
- [`2026/20260606120000_add_org_expense_summary/migration.sql`](20260606120000_add_org_expense_summary/migration.sql) — 적용 (선택)
- [`2026/20260606120000_add_org_expense_summary/rollback.sql`](20260606120000_add_org_expense_summary/rollback.sql) — **테이블 제거**

---

## 3. Ubuntu에 Go 설치 (미설치 시)

서버가 **Ubuntu**(22.04 / 24.04 LTS 등)이고 `go` 명령이 없을 때, 아래를 **SSH로 서버에 접속한 뒤** 한 번만 수행한다.  
레포의 `go.mod` 는 **Go 1.22.4** 이므로, **1.22 이상**이면 된다.

### 3.1 권장: go.dev 공식 바이너리 (버전 고정)

`apt install golang-go` 는 Ubuntu 버전에 따라 **1.18·1.21** 등으로 낮을 수 있어 **권장하지 않는다**. 공식 tarball 설치를 사용한다.

```bash
# 1) 기본 도구 (wget 없으면 curl 사용)
sudo apt update
sudo apt install -y wget ca-certificates git

# 2) 아키텍처 확인 (대부분 amd64, ARM 서버면 arm64)
uname -m
# x86_64  → GOARCH=amd64
# aarch64 → GOARCH=arm64

# 3) 버전·아키텍처 설정 (예: 1.22.5 + amd64)
GO_VERSION=1.22.5
GO_OS=linux
GO_ARCH=amd64   # ARM 이면 arm64

# 4) 다운로드·설치 (/usr/local/go)
cd /tmp
wget -q "https://go.dev/dl/go${GO_VERSION}.${GO_OS}-${GO_ARCH}.tar.gz"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "go${GO_VERSION}.${GO_OS}-${GO_ARCH}.tar.gz"
rm -f "go${GO_VERSION}.${GO_OS}-${GO_ARCH}.tar.gz"

# 5) PATH (로그인 셸마다 적용 — bash 기본)
grep -q '/usr/local/go/bin' ~/.profile 2>/dev/null || \
  echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
# 현재 SSH 세션에만 바로 반영
export PATH=$PATH:/usr/local/go/bin

# 6) 확인
go version
# go version go1.22.5 linux/amd64
```

| 확인 항목 | 기대 결과 |
|-----------|-----------|
| `go version` | `go1.22` 이상 |
| `which go` | `/usr/local/go/bin/go` |
| `go env GOPATH` | 기본값(예: `$HOME/go`) — 변경 불필요 |

첫 `go mod download` / `go run` 시 모듈·컴파일 캐시가 `$HOME/go` 아래에 생긴다(수 GB 여유 권장).

### 3.2 대안: Snap (빠른 설치, 버전은 채널에 따름)

```bash
sudo snap install go --classic
go version   # 1.22+ 인지 반드시 확인
```

팀 정책상 Snap을 쓰지 않으면 **§3.1** 만 사용한다.

### 3.3 (선택) Go 없이 바이너리만 서버에 두기

서버에 Go를 설치하지 않으려면, **로컬 Mac/Linux** 에서 Linux용 바이너리를 빌드해 전송할 수 있다.

```bash
# 로컬 — onfinance 레포 루트
GOOS=linux GOARCH=amd64 go build -o onfinance-import-linux-amd64 .
scp onfinance-import-linux-amd64 USER@SERVER:/opt/kcpc/onfinance/bin/onfinance-import
```

이 경우 `run_all_imports.sh` 는 여전히 내부에서 `go run` 을 호출하므로, **스크립트 경로를 그대로 쓰려면 서버에 Go 설치(§3.1)가 필요**하다. 바이너리만 쓸 때는 가이드 **§7.5** 처럼 직접 실행하거나, 팀에서 래퍼를 따로 두는 방식이 된다. **일반 운영은 §3.1 설치를 권장**한다.

### 3.4 설치 후 바로 할 일

```bash
go version
# 이후 §5(레포 clone) → §5.2(go mod download) 순서
```

문제 해결은 **§11** 의 `go: command not found` 항목을 참고한다.

---

## 4. 서버 디렉터리 구성 (권장)

```text
/opt/kcpc/onfinance/          # 또는 $HOME/onfinance (git clone)
├── current/                   # 적재용 엑셀 (로컬에서 업로드)
│   ├── GeneralLedger.*.xlsx
│   ├── FundActivitySummary.*.xlsx
│   └── GeneralLedger.Special.*.xlsx
├── scripts/
│   ├── run_all_imports.sh
│   ├── run_all_imports.env    # 서버 전용 (git 제외, chmod 600)
│   └── lib/
├── main.go, fund_activity_*.go, go.mod, go.sum
└── bin/                       # 선택: go build 출력
    └── onfinance-import
```

`current/`·`*.xlsx`·`run_all_imports.env` 는 레포 `.gitignore` 대상 — **서버에서 직접 생성·업로드**한다.

---

## 5. 최초 설치 (서버에서 1회)

**순서**: §3 Go 설치 → §5.1 레포 → §5.2 `go mod download` → §5.4 env

### 5.1 레포 받기

```bash
# 예: /opt/kcpc
sudo mkdir -p /opt/kcpc && sudo chown "$USER" /opt/kcpc
cd /opt/kcpc
git clone git@github.com:deokhokim911/kcpc_expense_import.git onfinance
cd onfinance
```

HTTPS/SSH는 환경에 맞게 선택. 이후 업데이트는 `git pull` 만으로 충분하다.

### 5.2 Go 모듈 의존성 (§3 설치 후)

```bash
cd /opt/kcpc/onfinance
go mod download
```

`go: command not found` 이면 **§3** 부터 진행한다.

### 5.3 (권장) 바이너리 빌드 — 반복 실행 시 `go run` 보다 빠름

```bash
mkdir -p bin
go build -o bin/onfinance-import .
```

이후 스크립트는 내부적으로 `go run .` 을 쓰므로, **네트워크가 느린 서버에서 매번 컴파일**을 피하려면 아래 §7.5 의 바이너리 직접 실행을 쓰거나, 환경변수로 바이너리 경로를 지정하는 방식을 팀에서 선택할 수 있다.  
**기능 변경 없이** 당장은 `go run` 그대로 `run_all_imports.sh` 를 실행해도 된다(첫 실행만 컴파일 시간 추가).

### 5.4 env 파일 (서버 전용)

```bash
cp scripts/run_all_imports.env_sample scripts/run_all_imports.env
chmod 600 scripts/run_all_imports.env
vi scripts/run_all_imports.env
```

**`DATABASE_URL` 은 서버에서 localhost 로** (예시 — 실제 비밀번호·DB명에 맞게 수정):

```bash
# Postgres가 같은 서버에서 5432 리스닝할 때
DATABASE_URL="postgres://USER:PASSWORD@127.0.0.1:5432/kcpc?sslmode=disable"

FISCAL_YEAR=2026

MINISTRY_LEDGER_XLSX=GeneralLedger.20260530.xlsx
FUND_SUMMARY_XLSX=FundActivitySummary.20260530.xlsx
FUND_DETAIL_LEDGER_XLSX=GeneralLedger.Special.20260530.xlsx

MINISTRY_IMPORT_QUIET=1
```

| 항목 | 설명 |
|------|------|
| `127.0.0.1` / `localhost` | DB 프로세스와 **동일 호스트**일 때 지연 최소 |
| 공인 IP (`69.x.x.x`) | 서버에서도 동작하지만, 루프백보다 불필요하게 느리거나 방화벽 경유 가능 |
| `?schema=public` | Prisma URL에 붙어 있어도 Go 쪽에서 제거 처리됨 |

연결 확인:

```bash
# psql 이 있으면
psql "$DATABASE_URL" -c 'SELECT 1'

# 또는 dry-run 만
./scripts/import_fund_activity_fiscal_year.sh --dry-run
```

---

## 6. 엑셀 파일 올리기 (로컬 → 서버)

로컬에서 QB export 후 `current/` 로 복사.

```bash
# 로컬 (Mac) 예시 — USER, HOST, 경로 수정
rsync -avz --progress \
  ./current/ \
  USER@DB_SERVER:/opt/kcpc/onfinance/current/
```

또는 파일 3개만:

```bash
scp current/GeneralLedger.*.xlsx \
    current/FundActivitySummary.*.xlsx \
    current/GeneralLedger.Special.*.xlsx \
  USER@DB_SERVER:/opt/kcpc/onfinance/current/
```

서버에서 파일명이 `run_all_imports.env` 와 일치하는지 확인:

```bash
ls -la /opt/kcpc/onfinance/current/
```

---

## 7. 실행 (로컬과 동일 명령)

서버에서:

```bash
cd /opt/kcpc/onfinance
chmod +x scripts/*.sh
```

### 7.1 미리보기만

```bash
./scripts/run_all_imports.sh --dry-run
```

### 7.2 실제 적재 (확인 프롬프트)

```bash
./scripts/run_all_imports.sh
# y 입력
```

확인 생략:

```bash
RUN_ALL_IMPORTS_YES=1 ./scripts/run_all_imports.sh
# 또는
./scripts/run_all_imports.sh --yes
```

### 7.3 Fund Activity 만 재적재

```bash
./scripts/import_fund_activity_fiscal_year.sh --dry-run
./scripts/import_fund_activity_fiscal_year.sh
```

### 7.4 사역원 org_balance 만

```bash
./scripts/import_ministry_org_balance.sh --dry-run
./scripts/import_ministry_org_balance.sh
```

모든 `import_*.sh` 는 `scripts/run_all_imports.env` 를 자동 로드한다.

### 7.5 (선택) 바이너리로 Go 호출 줄이기

스크립트 수정 없이 **일회성**으로 바이너리를 쓰려면, 동일 플래그를 직접 실행:

```bash
cd /opt/kcpc/onfinance
export $(grep -v '^#' scripts/run_all_imports.env | xargs)  # 주의: 값에 공백 있으면 source 권장
# 권장: source 방식
set -a && source <(tr -d '\r' < scripts/run_all_imports.env) && set +a
# lib 와 동일하게 경로 해석은 수동이므로, 보통은 run_all_imports.sh 사용이 안전

./bin/onfinance-import -replace-fund-activity-year -fiscal-year 2026 \
  -summary-xlsx current/FundActivitySummary.20260530.xlsx \
  -detail-ledger-xlsx current/GeneralLedger.Special.20260530.xlsx
```

일상 운영에서는 **`run_all_imports.sh` 유지**를 권장한다.

---

## 8. 코드 업데이트 (기능은 레포와 동일)

```bash
cd /opt/kcpc/onfinance
git pull
go mod download
# 바이너리 쓰는 경우
go build -o bin/onfinance-import .
```

엑셀·env 는 `git pull` 로 덮어쓰이지 않는다.

---

## 9. 보안·운영 체크리스트

| 항목 | 권장 |
|------|------|
| `run_all_imports.env` | `chmod 600`, 레포에 커밋 금지 |
| SSH | 키 인증, 필요 시 bastion 경유 |
| DB 계정 | 적재 전용 role, 최소 권한 |
| 방화벽 | 로컬 PC에서 **5432를 열 필요 없음** — SSH만 열리면 됨 |
| 로그 | `script import.log` 또는 `tee` 로 서버에 저장 |
| 백업 | fiscal-year replace 전 DB 스냅샷·백업 정책 팀 규칙 따름 |
| 기간 일치 | Summary·Special G/L 제목행 `Period … to …` **동일**해야 함 (로컬과 동일 검증) |

장시간 작업 시 세션 끊김 방지:

```bash
sudo apt install -y tmux   # 또는 screen
tmux new -s import
cd /opt/kcpc/onfinance && ./scripts/run_all_imports.sh
# Ctrl+B D 로 detach
```

---

## 10. 로컬 vs 서버 역할 분담 (권장 워크플로)

```mermaid
sequenceDiagram
  participant QB as QuickBooks
  participant Local as 로컬 PC
  participant Srv as DB 서버
  participant DB as Postgres

  QB->>Local: xlsx export
  Local->>Srv: scp/rsync → current/
  Local->>Srv: SSH
  Srv->>Srv: run_all_imports.sh --dry-run
  Srv->>DB: localhost (빠름)
  Srv->>Srv: 확인 후 live 적재
```

| 단계 | 어디서 |
|------|--------|
| Ubuntu Go 설치 (최초 1회) | **서버** — §3 |
| 엑셀 export | 로컬 (QB) |
| 파일 업로드 | 로컬 → 서버 `current/` |
| dry-run / 적재 | **서버** |
| 코드 수정·PR | 로컬 → `git push` → 서버 `git pull` |

---

## 11. 문제 해결

| 증상 | 확인 |
|------|------|
| `missing DB env vars` | 서버 `scripts/run_all_imports.env` 존재·`DATABASE_URL` |
| `파일 없음` | `current/` 에 파일명 일치, env 는 **파일명만** |
| `period mismatch` | Summary·G/L export 기간 종료일 동일하게 재 export |
| `unrecognized configuration parameter schema` | 최신 `main.go` pull (`sanitizePostgresDatabaseURL`) |
| 여전히 느림 | DB가 **다른 호스트**인지 확인 — URL을 DB 실제 호스트(사설 IP)로, 앱 서버와 DB 서버가 같을 때만 `127.0.0.1` |
| `go: command not found` | **§3** Ubuntu Go 설치. `which go` 가 비어 있으면 `source ~/.profile` 또는 재접속 |
| `go: downloading …` 이 오래 걸림 | 서버에서 첫 빌드·모듈 다운로드 — 정상. 이후는 캐시로 빨라짐 |

### DB가 별도 DB 전용 서버인 경우

- **적재 실행 호스트**: DB 서버에 Go 설치 + clone (가장 빠름), 또는
- **애플리케이션 서버**에서 DB **사설 IP**로 `DATABASE_URL` (같은 VPC면 로컬 대비 훨씬 빠름), 또는
- **SSH 터널 없이** 원격 공인 IP — 로컬과 비슷하게 느릴 수 있음

---

## 12. 요약

- **기능 변경 없음** — `run_all_imports.sh` / `import_*.sh` / Go 플래그 동일.
- **Ubuntu 서버**: **§3** 으로 Go 1.22+ 설치 → **§5** 레포·env → **§6** 엑셀 업로드 → **§7** 적재.
- **서버에 레포 + `current/` 엑셀 + `run_all_imports.env`(localhost DB)** 만 준비.
- 로컬은 export·업로드·SSH만 하고, **적재는 서버에서** 실행하면 네트워크 병목을 피할 수 있다.

---

## 13. 관련 문서

- [`DEVLOG_2026-05-19.md`](DEVLOG_2026-05-19.md) — 파서·`income_expense_kind` 등
- [`Import_UI_일괄적재_개발방안.md`](Import_UI_일괄적재_개발방안.md) — 향후 UI (별도)
- [`MinistryLedger_org_balance_적재_스크립트_계획.md`](MinistryLedger_org_balance_적재_스크립트_계획.md)
