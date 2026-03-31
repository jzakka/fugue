## Why

코드베이스에 린터/포맷터가 없어서 스타일 불일치가 커밋마다 쌓일 수 있다. pre-commit hook으로 커밋 시점에 자동 검사하여 코드 품질을 초기부터 잡는다. Go API는 이미 스캐폴드되어 있고, Next.js 웹은 곧 생성될 예정이므로 지금 세팅하는 게 적절하다.

## What Changes

- Go: `golangci-lint` 설정 추가 (`.golangci.yml`)
- Go: `gofmt`/`goimports` 자동 포맷
- Frontend(예정): ESLint + Prettier 설정 (Next.js 스캐폴드 시 함께 적용)
- pre-commit hook: `lefthook` 도입으로 커밋 시 자동 lint/format 실행
- Makefile에 `make lint`, `make fmt` 타겟 추가

## Capabilities

### New Capabilities
- `pre-commit-lint`: lefthook 기반 pre-commit hook 설정과 Go/Frontend 린트·포맷 도구 구성

### Modified Capabilities

(없음)

## Impact

- `apps/api/`: golangci-lint 설정, Makefile 타겟 추가
- 루트: `lefthook.yml` 추가
- 개발자 온보딩: `lefthook install` 한 번 실행 필요 (Makefile에 포함)
- CI: 향후 GitHub Actions에서 동일 lint 명령 재사용 가능
