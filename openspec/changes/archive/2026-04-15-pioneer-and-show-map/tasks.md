## 1. AI CLI 클라이언트 비인터랙티브 모드

- [x] 1.1 `CLIClient.Call`에서 command가 "codex"일 때 args 앞에 `["exec", "-"]`를 주입
- [x] 1.2 codex exec 주입 동작 및 비-codex 명령 비주입 동작에 대한 테스트 추가/업데이트

## 2. show-map DB 기반 스크립트 판정

- [x] 2.1 `bot.sql`에 `ListScriptKeysForGraph` 쿼리 추가
- [x] 2.2 `sqlc generate` 실행하여 Go 코드 생성
- [x] 2.3 `repository.go`의 `FetchGraphData`에서 `ListScriptKeysForGraph`로 스크립트 맵 구성
- [x] 2.4 `HasScript` 판정을 스크립트 맵 조회로 변경
- [x] 2.5 `CheckScriptExists` 함수 및 `ScriptPathTemplate` 상수 제거
- [x] 2.6 미사용 `os` import 제거

## 3. 검증

- [x] 3.1 `go build ./...` 통과
- [x] 3.2 `go test ./...` 전체 통과
