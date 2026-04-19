## Why

Archive된 `2026-04-18-scheduler-retry-backoff` change의 `tasks.md` 6.6이 유일하게 미완료(`[ ]`) 상태로 남아 있다. 이 태스크는 backoff 경계 검증을 위해 `scheduler-claim-api`의 성공 경로(`SetStatus("fetched")`)가 `fetch_error_count=3 → 0` 리셋과 `last_fetched_at` 비-NULL 세팅을 실제로 수행함을 통합 테스트로 확인하는 항목이며, 당시 의존하던 `scheduler-claim-api`는 이미 머지되었다(commit `0a50180`). 현재 코드는 해당 시나리오 중 `fetch_error_count = 0` 리셋까지만 단언하고 `last_fetched_at != NULL` 단언을 누락하고 있어, 행위 계약 경계가 테스트로 완전히 닫히지 않은 상태다. 대칭적으로 Harvester 측 `harvested_at != NULL` 단언도 점검이 필요하다.

## What Changes

- Pioneer 성공 경로 통합 테스트(`TestIntegration_SetStatus_FetchedExcludesFromQueue`)에 `last_fetched_at` 가 NULL이 아님을 확인하는 단언을 추가한다.
- Harvester 성공 경로 통합 테스트(`TestIntegration_SetStatus_HarvestedEmptyPinsSkipsInsert` 및 관련)에 `harvested_at` 가 NULL이 아님을 확인하는 단언 유무를 점검하고, 누락 시 동일하게 보강한다.
- Archive된 `2026-04-18-scheduler-retry-backoff/tasks.md`의 6.6을 `[x]` 로 전환하고, 커버되는 테스트의 위치 포인터를 주석으로 명시한다. (Fugue 저장소 내 명문화된 "archive immutable" 컨벤션은 현재 확인되지 않았으며, 이전 archive들의 내용적 봉인성과 균형을 맞추기 위해 본 change가 예외적으로 수행하는 "archive 내 체크박스 전환 + 포인터 주석" 수준으로 변경 범위를 한정한다. 문구/요구사항/스코프 변경은 금지한다.)
- `scheduler` capability의 두 기존 시나리오("fetched status — error_count 리셋", "harvested status — 빈 pinIDs")의 THEN 절을 **타임스탬프 비-NULL 단언까지 포함하도록 미세 강화**한다. 기존 THEN 절은 카운트 리셋/pin 매핑만 명시하여 동일 시나리오가 검증해야 할 타임스탬프 관찰 포인트(`last_fetched_at`, `harvested_at`)가 일반 시나리오("fetched status 처리", "harvested status 처리 — pin 매핑 INSERT")에 분리되어 있어, 해당 분기 시나리오를 그대로 테스트로 번역했을 때 타임스탬프 관찰이 누락될 여지가 있다. 행위 변경(behavior change)은 아니며, 관찰 포인트를 해당 시나리오 내에서 자족적(self-contained)으로 기술하는 문구 보강에 한정된다.

## Capabilities

### New Capabilities
(없음)

### Modified Capabilities
- `scheduler`: 두 시나리오 THEN 절에 타임스탬프 비-NULL 관찰 포인트를 명시적으로 추가하는 문구 보강(behavior 변경 아님). 상세는 `specs/scheduler/spec.md` 참조.

## Impact

- **코드**: `apps/api/internal/scheduler/postgres_scheduler_test.go` 의 Pioneer 성공 경로 통합 테스트(`TestIntegration_SetStatus_FetchedExcludesFromQueue`)에 `last_fetched_at` 비-NULL 단언 1건 추가. Harvester 대칭 테스트(`TestIntegration_SetStatus_HarvestedEmptyPinsSkipsInsert`, `TestIntegration_SetStatus_HarvestedAtomicityOnPinFKFail`)는 이미 `harvested_at.Valid` / `!harvested_at.Valid` 단언을 포함하고 있음을 확인하며, 누락 시에만 보강한다(조건부). 구현 파일(`url_scheduler.go`, `postgres_scheduler.go`, `backoff.go`)은 변경하지 않는다.
- **스펙**: 변경 없음. 요구사항은 이미 충분하다.
- **DB**: 변경 없음.
- **Archive 문서**: `openspec/changes/archive/2026-04-18-scheduler-retry-backoff/tasks.md`의 6.6 체크박스만 전환 및 포인터 주석 추가. 문구/요구사항은 수정하지 않는다.
- **범위 외**: `TestIntegration_Dequeue_LeaseExpiryReclaims` 전체-스위트 실행 시 test isolation으로 인한 flakiness, backoff 정책 수치 조정, `ErrorKind` enum 확장은 모두 본 change 밖이며 필요 시 별도 change로 분리한다.
