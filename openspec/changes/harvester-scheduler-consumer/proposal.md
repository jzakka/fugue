## Why

기존 `Harvester`(`apps/api/internal/bot/harvester.go`)는 사이트당 한 번 실행되는 단일 프로세스 BFS 워크플로(in-memory `BFSQueue`, `visited` map, `nodeMap` 캐시)를 전제로 동작한다. 이 구조는 `scheduler-frontier-table`/`scheduler-claim-api`로 도입되는 영속 frontier(`bot_frontier`)와 양립하지 않는다. 복수 워커가 같은 사이트를 처리하면 동일 노드를 중복 harvest하거나 한 워커가 죽으면 진행 상태가 통째로 휘발된다. Harvester를 `URLScheduler` consumer로 재정의해야 수평 확장과 재시작 안전성이 동시에 확보된다.

또한 사용자 강조 사항: **Harvester도 BFS가 아니라 우선순위 큐 기반**이어야 한다. "Pioneer는 BFS, Harvester는 별도 큐"가 아니라 양쪽 모두 동일 frontier에서 score 우선순위로 dequeue하며, "다음에 처리할 노드"는 인메모리 큐 자료구조가 아니라 `bot_frontier`의 partial index가 결정한다.

## What Changes

- 새 capability spec `harvester` 도입. Harvester를 "URLScheduler consumer"로 정의한다.
- Harvester 메인 루프를 다음 한 줄로 환원: `scheduler.Dequeue → fetch → ParseDocument → Index → Pin 생성 시 pin_id 갱신 (실패 시 harvest_error_count 증가)`.
- **BFS / `BFSQueue` / `visited` map / `nodeMap` 인메모리 상태 전면 제거.** "다음 노드"는 항상 `URLScheduler.Dequeue`로 결정한다.
- Harvester는 `pin_id IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now()` 조건의 partial index를 통해 claim한다 (실제 SQL/락 정의는 `scheduler-claim-api` 책임).
- 다중 워커 정확성(같은 row 중복 dequeue 금지)은 Harvester가 직접 구현하지 않고 `URLScheduler`가 제공하는 `FOR UPDATE SKIP LOCKED` 기반 claim에 위임한다.
- 성공 시 `scheduler.SetStatus(key, pin_id=...)` 또는 `frontier` row 갱신으로 `pin_id`를 채운다. 실패 시 `harvest_error_count`를 증가시키고 `next_harvest_at`을 백오프 시각으로 갱신한다 (백오프 정책 자체는 별도 change).
- **BREAKING**: 기존 bot spec의 Harvester BFS 전제 requirement(`Harvester 실행 완료 시 전체 통계를 집계한다` 중 BFS 의존 부분, `Harvester가 실제 HTML을 가져온다` 중 "노드 그래프 순회" 전제)를 새 `harvester` spec의 consumer 동작으로 대체한다. 단, "Harvester가 실제 HTML을 가져온다", "Pin을 DB에 생성한다", "콘텐츠 항목 중복 체크" 등 fetch/index 단위 동작은 본 change 범위 외이므로 수정하지 않는다.

## Capabilities

### New Capabilities
- `harvester`: URLScheduler에서 claim한 한 URL을 fetch → ParseDocument → Index → frontier row 갱신하는 단일 consumer 루프 정의. BFS / 인메모리 큐 / 사이트별 nodeMap 사용 금지. 다중 워커 정확성은 scheduler에 위임.

### Modified Capabilities
- `bot`: Harvester를 "그래프 BFS 순회자"에서 "scheduler consumer"로 재정의하는 데 따라 다음 requirement를 제거한다. 동등한 행위는 새 `harvester` spec에서 frontier 기반으로 정의된다.
  - "Harvester 실행 완료 시 전체 통계를 집계한다" (사이트 단위 BFS 종료 시점에 누적 통계를 산출하던 정의)
  - "Harvester CLI가 실제 모드를 지원한다" 중 사이트 ID 단일 인자로 BFS를 트리거하던 부분 (CLI 진입 자체는 유지하되, 동작은 scheduler consumer 루프로 대체)

## Impact

- **코드**: `apps/api/internal/bot/harvester.go`의 `harvestBFS`, `BFSQueue`, `visited`, `nodeMap`, `findRootNode` 의존 경로 제거. 신규 `Run(ctx)` 루프는 `URLScheduler.Dequeue → fetchHTMLShared → ParseDocument(스크립트 실행) → pipeline.Process → scheduler.SetStatus` 흐름. `apps/api/internal/bot/bfs_queue.go`는 Harvester 관점에서는 미사용 (Pioneer 측 처리는 별도 change).
- **DB**: 본 change 자체는 새 컬럼/인덱스를 추가하지 않는다. `scheduler-frontier-table`이 정의한 `bot_frontier.pin_id`, `harvest_error_count`, `next_harvest_at`과 Harvester partial index에 의존한다.
- **인터페이스**: `URLScheduler`(scheduler-claim-api에서 정의 예정: `Enqueue`, `Dequeue`, `SetStatus`)에 의존. 본 spec은 Harvester가 이 인터페이스를 어떻게 사용하는지를 규정한다.
- **운영**: Harvester를 N개 워커로 배포해도 동일 row가 중복 처리되지 않는다. 단일 워커 재시작 시에도 진행 상태가 frontier에 남아 있어 처음부터 다시 시작하지 않는다.
- **범위 외 (별도 change)**: Pin 문서 스키마 정의(`harvester-pin-document`), 워커 종료 조건/예산(`harvester-worker-budget`), 이미지/HTML 캐시, snapshot first-fetch 등은 본 change에서 다루지 않는다.
