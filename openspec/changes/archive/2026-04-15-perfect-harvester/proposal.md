## Why

Harvester CLI(`fuguebot harvester pixiv`)가 현재 MockScriptExecutor와 MockPipeline을 사용하고 있어, Pioneer가 생성한 실제 스크립트를 실행하지 않고 더미 데이터만 반환한다. pixiv용 AI-generated 스크립트가 DB에 저장되어 있지만 실제 HTML을 파싱해 RawItem을 추출하고, 미디어를 다운로드해 Pin으로 저장하는 end-to-end 파이프라인이 동작하지 않는다.

## What Changes

- **실제 ScriptExecutor 구현**: DB에 저장된 JavaScript 파싱 스크립트를 실행하여 HTML에서 RawItem(title, description, mediaURL, sourceURL, mediaType)을 추출하는 프로덕션 executor 구현
- **실제 Pipeline 구현**: RawItem을 받아 중복 체크 → 미디어 다운로드(S3) → Pin 생성까지의 프로덕션 파이프라인 구현
- **Harvester CLI 연결**: `fuguebot harvester` 커맨드에서 Mock 대신 실제 executor/pipeline 사용
- **Harvester 실행 결과 리포트**: 실행 후 추출/중복/실패 통계를 로그로 출력

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities
- `bot`: 프로덕션 ScriptExecutor(goja) 구현, 프로덕션 Pipeline(dedup/download/pin) 구현, Harvester CLI 연결

## Impact

- **코드**: `apps/api/internal/bot/` — 새 executor, pipeline 구현체 추가
- **코드**: `apps/api/cmd/bot/main.go` — harvester 커맨드에서 실제 구현체 사용
- **의존성**: JavaScript 실행 런타임 필요 (goja 등 Go-embedded JS engine)
- **DB**: `pins` 테이블에 실제 데이터 write 발생
- **스토리지**: S3/MinIO에 미디어 파일 업로드 발생
