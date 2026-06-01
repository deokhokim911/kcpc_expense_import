# Import UI — 로컬 데스크톱 앱 개발 방안

**작성일**: 2026-05-08  
**상위 문서**: [`Import_UI_일괄적재_개발방안.md`](Import_UI_일괄적재_개발방안.md) — **파일 지정 = 업로드(확정)** 를 따른다.

로컬 PC에서 **API 서버 없이** Postgres에 직접 연결하고, 업로드한 xlsx를 **앱 전용 스테이징 폴더**에 둔 뒤 dry-run → 확인 → 적재한다.

---

## 1. 업로드 in 로컬 앱

웹과 동일 UX, 저장 위치만 다름.

| 단계 | 동작 |
|------|------|
| 선택 | 드래그앤드롭 또는 Wails `OpenFileDialog` |
| 스테이징 | `~/.kcpc/import-ui/jobs/{jobId}/` 에 **복사** (원본은 그대로) |
| Go | 스테이징 **절대 경로** 3개 + `fiscalYear` → `importui.Preview` / `Execute` |

서버 경로 텍스트 입력 필드는 **두지 않는다.**

---

## 2. 권장 스택 (로컬 전용)

| 레이어 | 기술 | 버전 | 비고 |
|--------|------|------|------|
| Shell | **Wails** | v2.9+ | Go + WebView 단일 `.app` |
| UI | React + TypeScript | 19 / 5.7 | `desktop/frontend` |
| 빌드 | Vite | 6.x | Wails embed |
| 스타일 | Tailwind CSS | 4.x | `@tailwindcss/vite` |
| 컴포넌트 | shadcn/ui + Radix | latest | Dialog, Table, Progress |
| 업로드 UI | react-dropzone | v14 | 3슬롯 |
| 서버 상태 | TanStack Query | v5 | preview/execute |
| 검증 | Zod | v3 | PreviewResult |
| 애니메이션 | Framer Motion | 11.x | 단계 전환 |
| Backend | Go (`onfinance`) | 1.22+ | **in-process** |
| DB | lib/pq | 1.10+ | `DATABASE_URL` |
| xlsx | excelize | v2.8+ | 기존 |
| jobId | google/uuid | — | 스테이징 폴더명 |

### 2.1 Wails 바인딩 (HTTP API 대체)

```go
// app.go — Wails에 노출
func (a *App) StageUploadedFiles(jobId string, files StagedFilesInput) error
func (a *App) PreviewImport(params ImportParams) (PreviewResult, error)
func (a *App) ExecuteImport(jobId, confirmToken string) (ExecuteResult, error)
func (a *App) TestDatabaseConnection(url string) error
func (a *App) GetConfig() (AppConfig, error)
func (a *App) SaveConfig(cfg AppConfig) error
```

프론트:

```ts
import { PreviewImport } from '../wailsjs/go/main/App'
```

### 2.2 디렉터리 구조

```
onfinance/
  cmd/import-desktop/main.go
  internal/importui/
    service.go
    store.go      # ~/.kcpc/import-ui/jobs
    types.go
  desktop/frontend/   # Vite React
    src/pages/ImportWizard.tsx
    src/components/UploadSlot.tsx
```

### 2.3 설정 저장

| 키 | 저장소 |
|----|--------|
| `DATABASE_URL` | `~/.kcpc/import-ui/config.json` (`0600`) |
| Phase 2 | macOS Keychain 래퍼 |

---

## 3. UI 방향

- 배경 `#F7F6F3`, 카드 화이트, 포인트 `#0F766E`, 삭제 경고 앰버  
- 3단: 연결·연도 → **업로드 3슬롯** → 미리보기 → 확인 모달  
- 상세: [일괄적재 문서 §4](Import_UI_일괄적재_개발방안.md) Presentation 스택

---

## 4. 개발 명령

```bash
# 최초
go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd desktop/frontend && pnpm install

# 개발
cd cmd/import-desktop && wails dev

# 배포 (macOS)
wails build -platform darwin/arm64
```

---

## 5. 웹 Admin과의 관계

| | 로컬 Wails | 웹 Next |
|--|------------|---------|
| 업로드 | `~/.kcpc/.../jobs` | `IMPORT_DATA_ROOT/imports` |
| 통신 | Wails bind | Route Handlers |
| **공유** | `internal/importui` + **동일 React UI** (권장) |

---

## 6. 로드맵

1. `internal/importui` + staging store  
2. Wails + UploadSlot 3개 + Preview 테이블  
3. Execute + 로그 패널 + `wails build`  

---

## 7. 요약

로컬 앱도 **경로 입력이 아니라 업로드(스테이징 복사)** 이다. 기술 스택은 **Wails + React + shadcn + react-dropzone + Go importui 직접 호출**이다.
