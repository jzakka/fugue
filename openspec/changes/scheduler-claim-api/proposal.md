## Why

`scheduler-frontier-table` change에서 도입한 `bot_frontier` 테이블은 영속 저장소만 제공할 뿐, Pioneer/Harvester가 실제로 URL을 enqueue/dequeue/상태갱신하기 위한 **계약(interface)** 과 **claim 쿼리 규약**이 없다. 또한 `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue` 의사코드는 단일 프로세스 인메모리 구조를 가정하여 복수 워커 환경에서 동일 URL을 중복 dequeue할 위험이 있고, "block-on-empty"·"linearizable" 같은 핵심 의미가 코드 주석으로만 흩어져 있다. 본 change는 이 두 공백을 메워, Pioneer/Harvester가 의존할 안정적 큐 API를 스펙화한다.

## What Changes

- `URLPriorityQueue`(의사코드)를 **`URLScheduler`** 로 rename하고 정식 Go interface로 확정한다. 메서드 시그니처는 `Enqueue(urls ...string)`, `Dequeue(cond queryCondition) string`, `SetStatus(key string, status string)`.
- `Dequeue`는 **block-on-empty** 의미를 가진다. 큐가 비어 있으면 호출이 즉시 반환하지 않고, **busy-wait + `time.Sleep(1 * time.Second)`** 폴링 루프로 새 row가 claim 가능해질 때까지 기다린다.
- `Dequeue`는 **linearizable** 해야 한다. 동일 row가 두 워커에 동시에 dequeue되지 않도록 Postgres `SELECT ... FOR UPDATE SKIP LOCKED` 패턴으로 구현한다.
- `Dequeue`의 인자는 단순 status 문자열이 아니라 **`queryCondition`** (쿼리 빌더에 주입할 조건 객체)이다. Pioneer는 "fetch 안 된 row" 조건, Harvester는 "harvest 안 된 row" 조건을 각자 구성하여 동일 스케줄러를 호출한다. 즉 `status`는 **필터가 아니라 어떤 partial index를 탈지 결정하는 쿼리 조건**이다.
- `Enqueue`는 `bot_frontier`에 **upsert** (`INSERT ... ON CONFLICT (normalized_url) DO NOTHING` 또는 score 갱신)로 동작한다. 동일 URL 중복 enqueue가 DB unique 제약과 충돌하지 않도록 application 레벨에서도 보장한다.
- `SetStatus(key, status)`는 fetch/harvest 결과를 frontier row에 반영하는 **에러/완료 보고 채널**이다. 성공 시 `last_fetched_at` / `pin_id` 갱신, 실패 시 `*_error_count` 증가. 구체 backoff 산식은 본 change 범위 외(`scheduler-retry-backoff`).
- **BREAKING (의사코드)**: `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue`는 본 change에서 `URLScheduler`로 rename되고, `Dequeue(string)` 시그니처는 `Dequeue(queryCondition)`로 바뀐다. 호출부(Pioneer/Harvester 의사코드) 갱신 필요.
- 레거시 인메모리 큐 `apps/api/internal/bot/priority_queue.go`는 본 change의 후속 정리 단계에서 제거 대상으로 지정한다(실제 삭제는 호출부 마이그레이션 완료 후).

## Capabilities

### New Capabilities
- `scheduler`: Pioneer/Harvester가 공유하는 URLScheduler claim API. 본 change에서는 interface 시그니처, Dequeue의 linearizability/block-on-empty 의미, Enqueue의 upsert 의미, SetStatus의 보고 의미, queryCondition의 역할을 정의한다. 테이블 스키마(`scheduler-frontier-table`), backoff 정책(`scheduler-retry-backoff`), host 동시성 제어(`scheduler-host-token-bucket`)는 별도 change에서 다룬다.

### Modified Capabilities
<!-- 본 change는 scheduler 신규 capability에 requirement를 추가하므로 modified 항목 없음. -->

## Impact

- **코드**: 새 패키지 `apps/api/internal/scheduler/` (또는 `apps/api/internal/bot/scheduler/`)에 `URLScheduler` interface와 Postgres 구현체를 추가. sqlc 쿼리(`enqueue`, `claim_pioneer`, `claim_harvester`, `report_fetch`, `report_harvest`) 신설.
- **DB**: 신규 마이그레이션 없음. `scheduler-frontier-table`이 만든 `bot_frontier`와 partial index를 그대로 사용.
- **호출부**: Pioneer/Harvester는 본 change 범위에서 직접 교체하지 않고, 후속 change(`harvester-scheduler-consumer`, `pioneer-*`)에서 `URLScheduler`로 전환. 본 change는 contract 정의가 목적이며 단독 머지 가능.
- **운영**: `Dequeue` busy-wait가 빈 큐 상태에서 1초당 한 번 SELECT를 수행하므로, 워커 N개 × 1 QPS의 idle load가 발생. 부하 모니터링 필요.
- **문서**: `docs/architecture.md`의 bot 섹션에 URLScheduler interface 한 단락 추가, `apps/api/fuguebot_pseudo.go` 주석에 rename 사실 반영.
