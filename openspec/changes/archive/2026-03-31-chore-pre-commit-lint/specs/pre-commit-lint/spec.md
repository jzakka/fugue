## ADDED Requirements

### Requirement: pre-commit hook이 커밋 시 자동 실행된다
lefthook 기반 pre-commit hook이 설치되어 있으면, `git commit` 시 자동으로 lint/format 검사를 실행한다. 검사 실패 시 커밋이 차단된다.

#### Scenario: Go 파일 커밋 시 lint 통과
- **WHEN** Go 파일을 수정하고 `git commit` 실행
- **THEN** golangci-lint가 staged Go 파일에 대해 실행되고, 위반 없으면 커밋 성공

#### Scenario: Go 파일 커밋 시 lint 실패
- **WHEN** lint 위반이 있는 Go 파일을 staged하고 `git commit` 실행
- **THEN** golangci-lint가 에러를 출력하고 커밋이 차단됨

#### Scenario: Go 외 파일만 커밋
- **WHEN** `.md` 등 Go 외 파일만 수정하고 `git commit` 실행
- **THEN** Go lint가 스킵되고 커밋 성공

### Requirement: Go 코드 자동 포맷
goimports가 Go 파일의 import 정렬과 코드 포맷을 자동 수행한다. pre-commit hook에서 포맷 후 자동 re-stage한다.

#### Scenario: 포맷되지 않은 Go 파일 커밋
- **WHEN** 포맷되지 않은 Go 파일을 staged하고 `git commit` 실행
- **THEN** goimports가 자동 포맷하고, 포맷된 파일이 re-stage되어 커밋에 포함

### Requirement: golangci-lint 설정 파일
`.golangci.yml`이 `apps/api/`에 존재하며, 기본 린터 세트(gofmt, goimports, govet, errcheck, staticcheck)가 활성화된다.

#### Scenario: golangci-lint 실행
- **WHEN** `apps/api/` 에서 `golangci-lint run ./...` 실행
- **THEN** `.golangci.yml` 설정에 따라 린트 검사가 수행됨

### Requirement: Makefile 타겟
`apps/api/Makefile`에 `lint`, `fmt`, `setup` 타겟이 추가된다.

#### Scenario: make lint
- **WHEN** `apps/api/`에서 `make lint` 실행
- **THEN** golangci-lint가 전체 Go 코드에 대해 실행됨

#### Scenario: make fmt
- **WHEN** `apps/api/`에서 `make fmt` 실행
- **THEN** goimports가 전체 Go 파일을 포맷함

#### Scenario: make setup
- **WHEN** `apps/api/`에서 `make setup` 실행
- **THEN** lefthook install이 실행되어 pre-commit hook이 등록됨

### Requirement: lefthook 설정 파일
프로젝트 루트에 `lefthook.yml`이 존재하며, Go lint/format 훅이 정의된다. 향후 frontend 훅 추가가 가능한 구조여야 한다.

#### Scenario: lefthook 설정 구조
- **WHEN** `lefthook.yml` 파일을 확인
- **THEN** `pre-commit` 섹션에 Go lint와 format 명령이 정의되어 있음

#### Scenario: frontend 훅 확장
- **WHEN** 향후 `apps/web/` 스캐폴드 후 frontend lint 훅을 추가
- **THEN** `lefthook.yml`의 `pre-commit.commands`에 새 항목만 추가하면 됨
