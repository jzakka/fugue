## Why

Pioneer는 현재 자체 인메모리 BFS 큐(`PriorityQueue`/`BFSQueue`)와 사이트 단위 `visited` 맵으로 크롤 상태를 관리한다. 이 구조는 단일 프로세스 전제 위에서만 동작하며, 복수 워커로 수평 확장하면 같은 URL을 중복 fetch하거나 사이트별 quota가 깨진다. `scheduler-frontier-table`이 `pioneer_frontier` / `harvester_frontier` 두 독립 테이블을, `scheduler-claim-api`가 `URLScheduler` 인터페이스(`Enqueue(QueueType, urls...)` / `Dequeue(QueueType)` / `SetStatus` / `RecordFetchError` / `RecordHarvestError`)를 도입한 만큼, Pioneer는 더 이상 자체 큐를 들고 있을 이유가 없다. 다만 baseline `Enqueue(QueueHarvester, urls...)`는 `snapshot_key`를 건드리지 않는 규약이므로(baseline scheduler spec 확정), Pioneer가 fetch 직후 snapshot_key를 함께 기록하려면 `EnqueueHarvester(url, snapshotKey)` 메서드가 별도로 필요하다 — 이 추가 메서드는 본 change가 scheduler spec delta로 도입한다. Pioneer를 **`pioneer_frontier`의 consumer이자 `pioneer_frontier` + `harvester_frontier`의 producer**(fanout B)로 재정의하여 인메모리 크롤 상태를 모두 제거한다.

## What Changes

- Pioneer를 `pioneer_frontier` consumer로 재정의: 메인 루프는 `scheduler.Dequeue(QueuePioneer) → fetch → snapshot 저장 → parse(링크 추출) → filter → scheduler.Enqueue(QueuePioneer, filteredURLs) → scheduler.EnqueueHarvester(url, snapshotKey) → scheduler.SetStatus(url, "fetched", nil)` 반복이다.
- **Fanout B**: Pioneer는 한 번의 fetch 결과를 두 방향으로 내보낸다.
  1. 추출된 새 링크는 `scheduler.Enqueue(QueuePioneer, filteredURLs)`로 `pioneer_frontier`에 다시 투입한다(다음 크롤 대상).
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
- **Feature flag 기반 롤백 (전환기 임시 장치)**: 전환 기간 동안 `BOT_PIONEER_SCHEDULER=false` 설정으로 구 BFS 경로로 즉시 복귀하는 롤백 수단을 구현 단계에서 제공한다(tasks.md 3.10 / 6.1 / 6.5 참조). 이 플래그는 영구 spec requirement가 아니라 **배포 전환기 동안만 유지되는 임시 장치**이며, 신규 경로 안정화 확인 후 tasks.md 6.5에서 제거된다. frontier 테이블은 롤백 시에도 삭제하지 않는다.
- **BREAKING**: `bot` spec에서 3개 requirement를 제거한다.
  1. "BFS로 사이트를 탐색한다 (Pioneer)" — Pioneer의 탐색 모델이 scheduler consumer 루프로 대체됨에 따른 제거.
  2. "스냅샷 저장 실패는 fail-open으로 처리한다" — snapshot 저장 실패를 fail-open(크롤 성공 취급)에서 fail-close(`SetStatus(fetch_failed) + RecordFetchError("network")`)로 **외부 관찰 가능한 행위 역전**. archived `pioneer-snapshot-storage`가 남긴 fail-open 회귀 테스트는 본 change 구현 단계에서 교체 필요.
  3. "Pioneer는 ParseLinks 후 FilterLinks를 거쳐 Enqueue한다" — 동일 책임을 새 `pioneer` capability가 이어받아 SSOT를 단일화.

## Capabilities

### New Capabilities
- `pioneer`: URLScheduler를 백엔드로 하는 새 Pioneer 동작 모델. Dequeue/fetch/snapshot/parse/filter/Enqueue(pioneer)/EnqueueHarvester 루프, scheduler 보고 책임(`SetStatus` + `RecordFetchError`), 인메모리 상태 금지, 링크 추출만 담당하는 책임 경계, fanout B 동작을 정의한다.

### Modified Capabilities
- `bot`: 3개 requirement를 제거한다. (a) "BFS로 사이트를 탐색한다 (Pioneer)" — 인메모리 BFS 전제 제거, 탐색 모델은 새 `pioneer` capability가 담당. 세부 scenario 처리: "DOM 기반 링크 추출"/"필터 체인을 통한 링크 필터링"은 `bot` spec의 독립 requirement가 이미 담당하므로 무영향, "복합 우선순위 계산"은 `pioneer-link-filter-policy`로 이관, "최대 노드 수 제한"은 워커 단위 예산(`pioneer-worker-budget`)·host 단위 속도 제한(`scheduler-host-token-bucket`)으로 관점 전환, "이미 방문한 링크/부모 관계 엣지 생성"은 graph edge 폐기로 의도적 미복원. (b) "스냅샷 저장 실패는 fail-open으로 처리한다" — 새 `pioneer` capability의 "실패 시 `SetStatus` + `RecordFetchError` 둘 다 호출" 규약으로 대체되며, 이는 외부 관찰 가능한 행위가 fail-open → fail-close로 역전되는 BREAKING. (c) "Pioneer는 ParseLinks 후 FilterLinks를 거쳐 Enqueue한다" — 동일 책임을 새 `pioneer` capability의 "FilterChain 호출은 Pioneer의 책임이다" requirement가 이어받아 SSOT를 단일화. 세부 scenario 매핑은 `specs/bot/spec.md` 델타 참조.
- `scheduler`: `URLScheduler` 인터페이스에 `EnqueueHarvester(url, snapshotKey) error` 메서드를 ADDED Requirement로 추가한다. baseline scheduler spec은 "Enqueue 경로에서 `snapshot_key`를 건드리지 않으며 snapshot_key 기록은 후속 change 책임"이라 명시했고(baseline `scheduler` spec의 "Enqueue는 url_hash 기준 upsert로 동작한다" requirement), 본 change가 그 후속 책임을 이행한다. 기존 `Enqueue(QueueHarvester, urls...)` 경로는 snapshot_key 미변경 규약을 그대로 유지한다. UPSERT 규약: 이미 harvested인 row에 대한 no-op, 미완료 row에 대한 snapshot_key/next_harvest_at/harvest_error_count 갱신.

## Impact

- **코드**: `apps/api/internal/bot/pioneer.go` 및 인메모리 큐(`priority_queue.go`, `bfs_queue.go`)는 본 change 적용 시 제거 대상. Pioneer.Run의 새 본문은 본 change `design.md` Decision 1의 pseudo-code(`func (p *Pioneer) Run` 루프)를 canonical reference로 삼아 구현하며, fetch 이후 `saveSnapshot` → `EnqueueHarvester` 단계가 추가된다. `apps/api/fuguebot_pseudo.go`의 Pioneer.Run은 부가 참고 자료일 뿐, 라인 번호에 의존하지 않는다.
- **의존성**: `scheduler-frontier-table`(두 테이블), `scheduler-claim-api`(`URLScheduler` 인터페이스 + `Dequeue`/`Enqueue`/`SetStatus`/`RecordFetchError`/`RecordHarvestError`), `pioneer-snapshot-storage`(`SnapshotStore.Put` + `SnapshotKey` 공개 함수), `pioneer-link-filter-policy`(FilterChain 필터 정의)에 의존한다. `EnqueueHarvester` 메서드는 baseline에 존재하지 않으며 본 change가 scheduler spec delta로 추가한다. backoff, host token bucket, worker exit는 각각 `scheduler-retry-backoff`, `scheduler-host-token-bucket`, `pioneer-worker-budget`에서 다룬다.
- **운영**: Pioneer를 N개 인스턴스로 배포해도 scheduler가 정확히-한 번 claim을 보장한다. 프로세스 재시작 시 frontier에서 자연 복구된다. 롤백 시 feature flag만 토글하면 구 BFS 경로로 전환된다.
- **문서**: `docs/architecture.md`에 Pioneer-Scheduler 관계 다이어그램 갱신 필요 (fanout B 경로 표기 포함).
