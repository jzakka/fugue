## Why

기존 `Harvester`(`apps/api/internal/bot/harvester.go`)는 사이트당 한 번 실행되는 단일 프로세스 BFS 워크플로(in-memory `BFSQueue`, `visited` map, `nodeMap` 캐시)를 전제로 동작한다. 이 구조는 `scheduler-frontier-table`이 도입하는 영속 frontier(`pioneer_frontier`/`harvester_frontier`)와 양립하지 않는다. 복수 워커가 같은 노드를 처리하면 동일 URL을 중복 harvest하거나, 한 워커가 죽으면 인메모리 진행 상태가 통째로 휘발된다. Harvester를 `URLScheduler` consumer로 재정의해야 수평 확장과 재시작 안전성이 동시에 확보된다.

또한 사용자 강조 사항: **Harvester도 BFS가 아니라 우선순위 큐 기반**이어야 한다. "다음에 처리할 URL"은 인메모리 큐 자료구조가 아니라 `harvester_frontier`의 partial index(`WHERE harvested_at IS NULL AND harvest_error_count < 5 ORDER BY score DESC, next_harvest_at ASC`)가 결정한다.

본 change는 "Harvester consumer는 `harvester_frontier`에서만 Dequeue하고, 각 URL은 snapshot(있으면) 또는 live fetch(없으면)로 HTML을 확보한 뒤 Pin으로 파싱하여 `harvester_frontier_pins` 매핑을 남긴다"는 역할을 명확히 고정한다.

## What Changes

- 새 capability spec `harvester` 도입. Harvester를 "URLScheduler consumer"로 정의한다.
- Harvester 메인 루프를 다음 한 줄로 환원:
  `scheduler.Dequeue(QueueHarvester) → fetchSnapshotOrLive → harvestPipeline.Process → createPins → scheduler.SetStatus(url, "harvested", pinIDs)`.
- URL 수급은 `scheduler.Dequeue(scheduler.QueueHarvester)`로만 이루어진다. 자체 BFS/`BFSQueue`/`visited`/`nodeMap`/사이트별 인메모리 캐시는 **전면 제거**한다.
- Harvester는 `harvester_frontier`의 partial index를 통해 claim한다. 실제 SQL/락 정의는 `scheduler-claim-api` 책임, 파티셜 인덱스 정의는 `scheduler-frontier-table` 책임.
- `snapshot_key`가 있으면 object storage에서 snapshot을 읽어 HTML로 사용하고, 없거나 miss이면 live HTTP fetch로 폴백한다. 실제 fetch 동작은 `harvester-snapshot-first-fetch` 책임이며, 본 change는 "snapshot-first 경로를 경유한다"는 호출 규약만 정의한다.
- Pin 파싱(`harvestPipeline.Process`)은 `harvester-pin-document` 책임. 본 change는 pipeline이 반환한 `PinDocument`를 받아 Pin을 생성하고 그 `pin_id`들을 `SetStatus`로 전달하는 흐름만 정의한다.
- 다중 Pin 처리: 한 URL이 N개의 Pin을 생성할 수 있다. `createPins`의 결과 `[]int64`를 `SetStatus(url, "harvested", pinIDs)`에 그대로 전달하면, scheduler 구현이 `harvester_frontier.harvested_at` UPDATE와 `harvester_frontier_pins` 일괄 INSERT를 **한 트랜잭션**에서 처리한다. `pinnable = false`거나 Pin이 0건이면 `pinIDs = nil`로 호출해 매핑 없이 완료 표시한다.
- 실패 처리: fetch/파싱/Pin 생성 중 어느 단계든 실패하면 `SetStatus(url, "harvest_failed", nil)`과 `RecordHarvestError(url, errorKind)`를 **둘 다** 호출한다(DECISIONS §3 "Consumer 호출 규약"). `errorKind`는 `"http_4xx" | "http_5xx" | "network" | "timeout" | "parse" | "pin_create"` 중 하나.
- 재harvest 없음: `harvester_frontier` UPSERT는 `WHERE harvested_at IS NULL` 가드를 가지므로(DECISIONS §8), Harvester 측은 이미 harvest된 URL을 다시 claim할 걱정을 할 필요가 없다.
- 폴링 책임: 빈 큐 polling은 `Dequeue` **내부 책임**이다(DECISIONS §3, 1초 sleep). Harvester consumer 루프는 sleep/backoff를 자체 수행하지 않는다.
- 워커 단위 동시성: `FOR UPDATE SKIP LOCKED` claim은 `scheduler-claim-api`가 보장. Harvester는 자체 advisory lock/분산 락을 쓰지 않는다.
- **BREAKING**: 기존 bot spec의 Harvester BFS/통계/CLI 관련 requirement를 본 change에서 직접 REMOVED delta로 포함한다 (DECISIONS §12).

## Capabilities

### New Capabilities
- `harvester`: `harvester_frontier`에서 `Dequeue(QueueHarvester)`로 claim한 한 URL을 snapshot-first fetch → `PinDocument` 파싱 → Pin 생성 → `SetStatus(url, "harvested", pinIDs)`로 `harvester_frontier_pins` 매핑까지 단일 consumer 루프로 정의. BFS/인메모리 큐/사이트별 nodeMap 사용 금지. 다중 워커 정확성은 scheduler에 위임.

### Modified Capabilities
- `bot`: 본 change의 `specs/bot/spec.md`에서 직접 REMOVED delta로 다음 requirement 2건을 제거한다. 동등한 행위는 새 `harvester` spec에서 frontier 기반으로 재정의된다.
  - "Harvester 실행 완료 시 전체 통계를 집계한다" (사이트 단위 BFS 종료 시점 누적 통계)
  - "Harvester CLI가 실제 모드를 지원한다" (사이트 ID 인자로 BFS 1회 트리거하는 CLI 모드)

## Prerequisites

본 change는 다음 change가 먼저 적용되어 있어야 의미가 있다:

- `scheduler-frontier-table`: `harvester_frontier`, `harvester_frontier_pins` 테이블과 partial index 정의.
- `scheduler-claim-api`: `scheduler.Dequeue(QueueType)`, `SetStatus(key, status, pinIDs)`, `RecordHarvestError(key, errorKind)` 인터페이스와 `FOR UPDATE SKIP LOCKED` claim 동작.
- `harvester-snapshot-first-fetch`: `snapshot_key` 기반 snapshot 조회 + miss 시 HTTP fallback 정의.
- `harvester-pin-document`: `harvestPipeline.Process(html) → PinDocument`의 스키마와 `Pinnable` 판정 규칙.

본 change의 tasks.md 1절(Prerequisites)도 동일 목록을 명시한다.

## Impact

- **코드**: `apps/api/internal/bot/harvester.go`의 `harvestBFS`, `BFSQueue`, `visited`, `nodeMap`, `findRootNode`, 사이트 단위 통계 집계 의존 경로 제거. 신규 `Run(ctx)` 루프는 본 spec의 pseudo-code대로 `scheduler.Dequeue(QueueHarvester) → fetchSnapshotOrLive → harvestPipeline.Process → createPins → scheduler.SetStatus` 흐름. `apps/api/internal/bot/bfs_queue.go`는 Harvester 관점에서 미사용(Pioneer 측 처리는 별도 change).
- **DB**: 본 change 자체는 새 컬럼/인덱스를 추가하지 않는다. `scheduler-frontier-table`이 정의한 `harvester_frontier`와 `harvester_frontier_pins`에 의존한다.
- **인터페이스**: `URLScheduler`(`scheduler-claim-api`: `Dequeue`, `SetStatus`, `RecordHarvestError` 등)에 의존. 본 spec은 Harvester가 이 인터페이스를 어떻게 사용하는지만 규정한다.
- **운영**: Harvester를 N개 워커로 배포해도 동일 URL이 중복 처리되지 않는다(scheduler 락 계약). 단일 워커 재시작 시에도 진행 상태가 `harvester_frontier`에 남아 있어 손실 없이 재개한다.
- **범위 외 (별도 change)**: Pin 문서 스키마 정의(`harvester-pin-document`), 이미지 캐시(`harvester-image-cache`), snapshot storage 키 규약(`pioneer-snapshot-storage`), snapshot-first fetch 폴백 규약(`harvester-snapshot-first-fetch`), 워커 종료 예산(`harvester-worker-budget`), 백오프 수식(`scheduler-retry-backoff`).
