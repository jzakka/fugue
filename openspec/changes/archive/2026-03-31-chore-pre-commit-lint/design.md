## Context

Fugue 프로젝트는 Go API(`apps/api/`)가 초기 스캐폴드 상태이고, Next.js 웹(`apps/web/`)은 아직 생성 전이다. 린터/포맷터/pre-commit hook이 전혀 없어서 코드 스타일 일관성을 보장할 수 없다. 초기 단계에서 세팅해야 나중에 마이그레이션 비용이 없다.

## Goals / Non-Goals

**Goals:**
- Go 코드에 golangci-lint 적용 (gofmt, goimports, vet 등 기본 린터)
- pre-commit hook으로 커밋 시점에 자동 lint/format 실행
- `make lint`, `make fmt` 로 수동 실행 가능
- 향후 Next.js 추가 시 ESLint/Prettier 훅 확장 용이한 구조

**Non-Goals:**
- CI/CD 파이프라인에 lint 단계 추가 (별도 change)
- Next.js ESLint/Prettier 설정 (웹 스캐폴드 시 함께 처리)
- 커스텀 lint 규칙 작성
- IDE 통합 설정 (.vscode/, .idea/)

## Decisions

### 1. pre-commit 도구: lefthook

Go 기반 바이너리라 Node.js 의존성 없이 설치 가능. Go 프로젝트와 기술 스택이 일치하고, YAML 설정이 간결하다.

**대안**: husky (Node.js 의존), pre-commit (Python 의존). 둘 다 추가 런타임 의존성이 필요해서 제외.

### 2. Go 린터: golangci-lint

업계 표준 메타 린터. gofmt, goimports, govet, errcheck, staticcheck 등을 한 번에 실행. `.golangci.yml`로 설정 관리.

**대안**: 개별 린터 직접 실행. 관리 포인트가 많아지고 버전 관리가 어려워서 제외.

### 3. 설정 파일 위치

- `lefthook.yml`: 프로젝트 루트 (모노레포 전체 관할)
- `.golangci.yml`: `apps/api/` 하위 (Go 프로젝트 스코프)
- Makefile 타겟: `apps/api/Makefile`에 추가 (기존 Makefile 확장)

### 4. goimports 활용

gofmt 대신 goimports를 포맷터로 사용. import 정렬까지 자동 처리되어 gofmt의 상위 호환.

## Risks / Trade-offs

- **[lefthook 설치 필요]** → `make setup` 타겟에 `lefthook install` 포함. README/CLAUDE.md에 온보딩 안내.
- **[pre-commit이 느릴 수 있음]** → lefthook은 변경된 파일만 검사하므로 빠름. glob 패턴으로 Go 파일만 대상.
- **[golangci-lint 버전 고정 필요]** → `.golangci.yml`에 최소 버전 명시, CI와 로컬 버전 일치 보장.
