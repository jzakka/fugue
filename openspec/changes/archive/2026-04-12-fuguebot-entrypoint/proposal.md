## Why

Fuguebot의 pioneer와 harvester를 로컬 개발 및 테스트 환경에서 쉽게 실행할 수 있어야 한다. 현재는 단일 진입점(`cmd/bot/main.go`)만 존재하여 특정 크롤러나 사이트를 대상으로 테스트하기 어렵다. Cobra 기반 CLI를 도입하여 `make pioneer <site>`, `make harvester <site>` 명령으로 각 크롤러를 독립적으로 실행할 수 있게 한다.

## What Changes

- `cmd/bot/main.go`를 Cobra CLI 구조로 전환
  - 루트 커맨드: 기본 정보 표시 및 서브커맨드 안내
  - `pioneer <site>` 서브커맨드: 지정된 사이트에 대해 Pioneer 크롤러 실행
  - `harvester <site>` 서브커맨드: 지정된 사이트에 대해 Harvester 크롤러 실행
  - Source name("unsplash", "fma")을 bot_sites 테이블의 domain과 매핑하는 registry 구현
- `apps/api/Makefile`에 `pioneer`, `harvester` 타겟 추가
  - `make pioneer <site>` → `go run cmd/bot/main.go pioneer <site>`
  - `make harvester <site>` → `go run cmd/bot/main.go harvester <site>`
- 각 서브커맨드는 필요한 인프라(DB, Storage)를 초기화하고 해당 크롤러 인스턴스를 생성하여 실행
- Source name을 domain으로 변환하는 매핑 로직 (예: "unsplash" → "unsplash.com", "fma" → "freemusicarchive.org")
- 사이트 이름 검증 및 존재하지 않는 사이트에 대한 오류 처리

## Capabilities

### New Capabilities
- `bot-cli-interface`: Cobra 기반 CLI 인터페이스로 fuguebot의 pioneer와 harvester를 독립적으로 실행

### Modified Capabilities

## Impact

- `apps/api/cmd/bot/main.go` 파일 구조 변경 (단일 main → Cobra CLI 구조)
- `apps/api/Makefile`에 새로운 타겟 추가
- 기존 bot 실행 방식과 호환성 유지 (기본 커맨드로 전체 크롤 실행 가능)
- Pioneer, Harvester 생성자 호출 코드 추가 필요
