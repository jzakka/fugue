## Why

Pioneer는 현재 자체 인메모리 BFS 큐(`PriorityQueue`/`BFSQueue`)와 사이트 단위 `visited` 맵으로 크롤 상태를 관리한다. 이 구조는 단일 프로세스 전제 위에서만 동작하며, 복수 워커로 수평 확장하면 같은 URL을 중복 fetch하거나 사이트별 quota가 깨진다. `scheduler-frontier-table`이 `pioneer_frontier` / `harvester_frontier` 두 독립 테이블을, `scheduler-claim-api`가 `URLScheduler` 인터페이스(`Enqueue(QueueType, urls)` / `Dequeue(QueueType)` / `SetStatus` / `RecordFetchError` / `EnqueueHarvester` 등)를 도입한 만큼, Pioneer는 더 이상 자체 큐를 들고 있을 이유가 없다. Pioneer를 **`pioneer_frontier`의 consumer이자 `pioneer_frontier` + `harvester_frontier`의 producer**(fanout B)로 재정의하여 인메모리 크롤 상태를 모두 제거한다.

## What Changes

- Pioneer를 `pioneer_frontier` consumer로 재정의: 메인 루프는 `scheduler.Dequeue(QueuePioneer) → fetch → snapshot 저장 → parse(링크 추출) → filter → scheduler.Enqueue(QueuePioneer, filteredLinks) → scheduler.EnqueueHarvester(url, snapshotKey) → scheduler.SetStatus(url, "fetched", nil)` 반복이다.
- **Fanout B**: Pioneer는 한 번의 fetch 결과를 두 방향으로 내보낸다.
  1. 추출된 새 링크는 `scheduler.Enqueue(QueuePioneer, filteredLinks)`로 `pioneer_frontier`에 다시 투입한다(다음 크롤 대상).
  2. **fetch된 원본 URL + `snapshot_key`**는 `scheduler.EnqueueHarvester(url, snapshotKey)`로 `harvester_frontier`에 UPSERT한다(`ON CONFLICT (url_hash) DO UPDATE ... WHERE harvested_at IS NULL`). 이미 harvest가 끝난 URL에 대해서는 no-op으로 동작한다.
- Pioneer는 자체 큐/BFS/`visited` 맵을 보유하지 않는다. URL 중복, 사이트 경계, 우선순위 정렬은 모두 `URLScheduler`(즉 `pioneer_frontier` / `harvester_frontier` 테이블 + partial index)가 담당한다.
- **Dequeue 시그니처**: `scheduler.Dequeue(scheduler.QueuePioneer)` 단일 인자. 과거 `queryCondition` 문자열은 제거되며, 폴링/빈 큐 sleep은 scheduler 내부 책임이다. Pioneer 루프에 별도 sleep을 두지 않는다.
- **fetch 보고 규약**:
  - 성공 시: `scheduler.SetStatus(url, "fetched", nil)` — `next_fetch_at`이 `now() + 365 days`로 갱신되며 `fetch_error_count`는 0으로 리셋된다(재크롤 정책은 후속 change).
  - 실패 시: `scheduler.SetStatus(url, "fetch_failed", nil)` + `scheduler.RecordFetchError(url, errorKind)` **둘 다 호출**한다. `errorKind`는 `"http_4xx"` / `"http_5xx"` / `"timeout"` / `"network"` 중 하나.
- **Filter 호출 책임**: 링크 필터링은 Pioneer consumer가 `FilterChain.Apply(links)`를 호출하여 수행한다. 필터 구성(DomainFilter / ExtensionFilter / PathPatternFilter / RobotsFilter / DedupFilter 등)의 구체 내용은 `pioneer-link-filter-policy`에서 정의하며, 본 change는 "호출 타이밍(Enqueue 직전)"만 확정한다.
- **교차 사이트 크롤을 허용한다**: Pioneer는 사이트 경계를 인지하지 않으며, 도메인 제한이 필요한 경우는 link filter(`pioneer-link-filter-policy`)에서 정책으로 다룬다.
- 콘텐츠 추출(스크립트 실행, Pin 생성)은 본 change 범위 밖이며 Harvester의 책임이다. Pioneer는 **링크 추출 + snapshot 저장 + harvester_frontier fanout**만 수행한다.
- 다중 워커 정확성(중복 fetch 방지, 정확히-한 번 claim)은 scheduler의 `FOR UPDATE SKIP LOCKED` + host token bucket 기반 claim이 보장한다. Pioneer 코드에 동시성 제어 로직을 두지 않는다.
- Pioneer 프로세스는 인메모리 크롤 상태(큐/visited/카운터)를 가져서는 안 된다(SHALL NOT). 재시작 후 곧바로 frontier 상태를 그대로 이어받을 수 있어야 한다.
- **Feature flag 기반 롤백**: 전환 기간 동안 `BOT_PIONEER_SCHEDULER=false`로 설정하면 구 BFS 경로로 즉시 복귀 가능해야 한다. frontier 테이블은 삭제하지 않는다.
- **BREAKING**: `bot` spec의 "BFS로 사이트를 탐색한다 (Pioneer)" requirement를 제거한다.

## Capabilities

### New Capabilities
- `pioneer`: URLScheduler를 백엔드로 하는 새 Pioneer 동작 모델. Dequeue/fetch/snapshot/parse/filter/Enqueue(pioneer)/EnqueueHarvester 루프, scheduler 보고 책임(`SetStatus` + `RecordFetchError`), 인메모리 상태 금지, 링크 추출만 담당하는 책임 경계, fanout B 동작을 정의한다.

### Modified Capabilities
- `bot`: 인메모리 BFS 전제를 가진 마지막 requirement("BFS로 사이트를 탐색한다 (Pioneer)")를 제거한다. 동작 재정의는 새 `pioneer` capability에서 한다.

## Impact

- **코드**: `apps/api/internal/bot/pioneer.go` 및 인메모리 큐(`priority_queue.go`, `bfs_queue.go`)는 본 change 적용 시 제거 대상. Pioneer.Run의 새 본문은 `apps/api/fuguebot_pseudo.go` Pioneer.Run(라인 33-68)을 정식 구현으로 옮긴 형태가 되며, fetch 이후 `saveSnapshot` → `EnqueueHarvester` 단계가 추가된다.
- **의존성**: `scheduler-frontier-table`(두 테이블), `scheduler-claim-api`(`URLScheduler` 인터페이스 + `EnqueueHarvester` + `RecordFetchError`), `pioneer-snapshot-storage`(`snapshot_key` 규약), `pioneer-link-filter-policy`(FilterChain 필터 정의)에 의존한다. backoff, host token bucket, worker exit는 각각 `scheduler-retry-backoff`, `scheduler-host-token-bucket`, `pioneer-worker-budget`에서 다룬다.
- **운영**: Pioneer를 N개 인스턴스로 배포해도 scheduler가 정확히-한 번 claim을 보장한다. 프로세스 재시작 시 frontier에서 자연 복구된다. 롤백 시 feature flag만 토글하면 구 BFS 경로로 전환된다.
- **문서**: `docs/architecture.md`에 Pioneer-Scheduler 관계 다이어그램 갱신 필요 (fanout B 경로 표기 포함).
