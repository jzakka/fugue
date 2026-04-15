## Why

Pioneer가 AI로 생성한 스크립트를 `bot_scripts` 테이블에 저장하지만, 두 가지 문제로 end-to-end 워크플로우가 동작하지 않는다. (1) AI CLI 클라이언트가 `codex`를 인터랙티브 모드로 실행하여 TTY가 없는 환경에서 실패한다. (2) `show-map` 시각화가 디스크 파일 존재 여부로 스크립트 구현 상태를 판단하여, DB에 저장된 스크립트를 인식하지 못한다.

## What Changes

- AI CLI 클라이언트가 `codex exec` 서브커맨드를 사용하여 비인터랙티브 모드로 동작하도록 수정
- `show-map` 시각화의 스크립트 존재 판정을 파일 시스템 기반에서 DB(`bot_scripts` 테이블) 기반으로 변경
- `CheckScriptExists` 파일 체크 함수 제거, DB 조회로 대체

## Capabilities

### New Capabilities

_(없음)_

### Modified Capabilities

- `bot`: Pioneer AI 클라이언트의 CLI 실행 방식 변경, show-map 시각화의 스크립트 존재 판정 로직 변경

## Impact

- `apps/api/internal/bot/ai/cli.go`: CLI 클라이언트의 `Call` 메서드에서 `codex exec -` 서브커맨드 자동 주입
- `apps/api/internal/bot/ai/cli_test.go`: codex exec 주입 동작 테스트 추가
- `apps/api/internal/bot/cmd/visualize/repository.go`: `CheckScriptExists` 파일 체크 제거, `ListScriptKeysForGraph` DB 조회로 대체
- `apps/api/db/queries/bot.sql`: `ListScriptKeysForGraph` 쿼리 추가
- `apps/api/internal/db/bot.sql.go`: sqlc 자동 생성 코드 변경
