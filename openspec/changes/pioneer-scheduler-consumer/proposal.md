## Why

Pioneer는 현재 자체 인메모리 BFS 큐(`PriorityQueue`/`BFSQueue`)와 사이트 단위 `visited` 맵으로 크롤 상태를 관리한다. 이 구조는 단일 프로세스 전제 위에서만 동작하며, 복수 워커로 수평 확장하면 같은 URL을 중복 fetch하거나 사이트별 quota가 깨진다. `scheduler-frontier-table`이 영속 frontier 테이블을, `scheduler-claim-api`가 `URLScheduler` 인터페이스(`Enqueue`/`Dequeue`/`SetStatus`)를 도입한 만큼, Pioneer는 더 이상 자체 큐를 들고 있을 이유가 없다. Pioneer를 **URLScheduler의 consumer이자 producer**로 재정의하여 인메모리 크롤 상태를 모두 제거한다.

## What Changes

- Pioneer를 `URLScheduler` consumer로 재정의: 메인 루프는 `scheduler.Dequeue → fetch → parse(링크 추출) → scheduler.Enqueue(urls)` 단순 반복이다.
- Pioneer는 자체 큐/BFS/`visited` 맵을 보유하지 않는다. URL 중복, 사이트 경계, 우선순위 정렬은 모두 `URLScheduler`(즉 frontier 테이블)가 담당한다.
- fetch 결과는 `scheduler.SetStatus(key, "fetched"|"error")`로 scheduler에 보고한다. 성공/실패 시맨틱은 scheduler가 frontier 컬럼(`last_fetched_at`, `fetch_error_count` 등)으로 영속화한다.
- Pioneer는 동시에 producer이다: 추출된 링크를 `scheduler.Enqueue(urls...)`로 다시 frontier에 넣는다(중복은 `normalized_url` unique constraint가 흡수).
- **교차 사이트 크롤을 허용한다**: Pioneer는 사이트 경계를 인지하지 않으며, 도메인 제한이 필요한 경우는 link filter(`pioneer-link-filter-policy`)에서 정책으로 다룬다.
- 콘텐츠 추출(스크립트 실행, Pin 생성)은 본 change 범위 밖이며 Harvester의 책임이다. Pioneer는 **링크 추출만** 수행한다.
- 다중 워커 정확성(중복 fetch 방지, 정확히-한 번 claim)은 scheduler의 `FOR UPDATE SKIP LOCKED` 기반 claim이 보장한다. Pioneer 코드에 동시성 제어 로직을 두지 않는다.
- Pioneer 프로세스는 인메모리 크롤 상태(큐/visited/카운터)를 가져서는 안 된다(SHALL NOT). 재시작 후 곧바로 frontier 상태를 그대로 이어받을 수 있어야 한다.
- **BREAKING**: `bot` spec의 "BFS로 사이트를 탐색한다 (Pioneer)" requirement를 제거한다. (인메모리 BFS 전제의 다른 3개 requirement는 `scheduler-frontier-table`에서 이미 제거됨)

## Capabilities

### New Capabilities
- `pioneer`: URLScheduler를 백엔드로 하는 새 Pioneer 동작 모델. Dequeue/fetch/parse/Enqueue 루프, scheduler 보고 책임, 인메모리 상태 금지, 링크 추출만 담당하는 책임 경계를 정의한다.

### Modified Capabilities
- `bot`: 인메모리 BFS 전제를 가진 마지막 requirement("BFS로 사이트를 탐색한다 (Pioneer)")를 제거한다. 동작 재정의는 새 `pioneer` capability에서 한다.

## Impact

- **코드**: `apps/api/internal/bot/pioneer.go` 및 인메모리 큐(`priority_queue.go`, `bfs_queue.go`)는 본 change 적용 시 제거 대상. Pioneer.Run의 새 본문은 `apps/api/fuguebot_pseudo.go` Pioneer.Run(라인 33-68)을 정식 구현으로 옮긴 형태가 된다.
- **의존성**: `scheduler-frontier-table`(테이블), `scheduler-claim-api`(`URLScheduler` 인터페이스)에 의존한다. backoff, host token bucket, worker exit, link filter, snapshot 정책은 각각 별도 change(`scheduler-retry-backoff`, `scheduler-host-token-bucket`, `pioneer-worker-budget`, `pioneer-link-filter-policy`, `harvester-snapshot-first-fetch`)에서 다룬다.
- **운영**: Pioneer를 N개 인스턴스로 배포해도 scheduler가 정확히-한 번 claim을 보장한다. 프로세스 재시작 시 frontier에서 자연 복구된다.
- **문서**: `docs/architecture.md`에 Pioneer-Scheduler 관계 다이어그램 갱신 필요.
