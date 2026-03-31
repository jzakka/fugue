## 1. golangci-lint 설정

- [x] 1.1 `apps/api/.golangci.yml` 생성 (gofmt, goimports, govet, errcheck, staticcheck 활성화)
- [x] 1.2 `apps/api/`에서 `golangci-lint run ./...` 실행하여 설정 검증

## 2. Makefile 타겟 추가

- [x] 2.1 `apps/api/Makefile`에 `lint` 타겟 추가 (`golangci-lint run ./...`)
- [x] 2.2 `apps/api/Makefile`에 `fmt` 타겟 추가 (`goimports -w .`)
- [x] 2.3 `apps/api/Makefile`에 `setup` 타겟 추가 (`lefthook install`)

## 3. lefthook 설정

- [x] 3.1 프로젝트 루트에 `lefthook.yml` 생성 (pre-commit 섹션에 Go lint + format 명령 정의)
- [x] 3.2 Go lint 훅: staged `.go` 파일 대상 golangci-lint 실행
- [x] 3.3 Go format 훅: staged `.go` 파일 대상 goimports 자동 포맷 + re-stage

## 4. 검증

- [x] 4.1 `lefthook install` 실행하여 hook 등록 확인
- [x] 4.2 포맷 위반 Go 파일 커밋 시 자동 포맷 + 커밋 성공 확인
- [x] 4.3 lint 위반 Go 파일 커밋 시 커밋 차단 확인
- [x] 4.4 Go 외 파일만 커밋 시 Go lint 스킵 확인
