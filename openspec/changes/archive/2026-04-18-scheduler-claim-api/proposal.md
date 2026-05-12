## Why

`scheduler-frontier-table` change에서 도입한 `pioneer_frontier` / `harvester_frontier` 테이블은 영속 저장소만 제공할 뿐, Pioneer/Harvester가 실제로 URL을 enqueue/dequeue/상태갱신하기 위한 **계약(interface)** 과 **claim 쿼리 규약**이 없다. 또한 `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue` 의사코드는 단일 프로세스 인메모리 구조를 가정하여 복수 워커 환경에서 동일 URL을 중복 dequeue할 위험이 있고, "block-on-empty"·"linearizable" 같은 핵심 의미가 코드 주석으로만 흩어져 있다. 본 change는 이 공백을 메워, Pioneer/Harvester가 의존할 안정적 큐 API를 스펙화한다.

## What Changes

- `URLPriorityQueue`(의사코드)를 **`URLScheduler`** 로 rename하고 정식 Go interface로 확정한다. 메서드 시그니처는 다음과 같이 단순화된다:
  - `Enqueue(queueType QueueType, urls ...string) error`
  - `Dequeue(queueType QueueType) (url string, err error)`
  - `SetStatus(key string, status string, pinIDs []uuid.UUID) error`
  - `RecordFetchError(key string, errorKind string) error`
  - `RecordHarvestError(key string, errorKind string) error`
- `QueueType` enum을 도입한다. 값은 `pioneer`, `harvester` 두 가지이며, Dequeue는 이 enum으로 어느 테이블/partial index를 대상으로 claim할지 결정한다. 기존 설계 초안에서 고려되었던 `queryCondition` 류의 WHERE 절 조립 추상(클로저·빌더·조건 객체)은 **본 change에서 도입하지 않는다**. 두 큐 경로만 있는 현 MVP 범위에서는 `QueueType` switch로 충분하며, 조건 조립 라이브러리에 의존하지 않는다.
- `Dequeue`는 **block-on-empty** 의미를 가진다. 큐가 비어 있거나 host throttle로 claim에 실패하면 호출이 즉시 반환하지 않고, **1초 고정 sleep 후 재시도**하는 폴링 루프로 claim 가능한 row가 등장할 때까지 기다린다. 빈 큐와 host throttle 미통과는 동일하게 1초 sleep으로 처리한다.
- `Dequeue`는 **linearizable** 해야 한다. 동일 row가 두 워커에 동시에 dequeue되지 않도록 Postgres `SELECT ... FOR UPDATE SKIP LOCKED` 패턴으로 구현한다.
- Claim 프로토콜은 host 동시성 제어와 통합된다: partial index ORDER BY로 상위 N개 후보(`SCHEDULER_CLAIM_CANDIDATE_N`, default 1)를 `FOR UPDATE SKIP LOCKED`로 잠그고, 각 row의 host에 대해 `HostRateLimiter.Allow(host)`를 호출하여 **처음 true를 반환한 row**를 claim한다. 모두 throttle이면 트랜잭션을 롤백하고 1초 sleep 후 재시도.
- **in-flight marker는 별도 컬럼을 두지 않는다**. Pioneer는 claim 직후 `next_fetch_at = now() + 10min`(lease 10분)을, Harvester는 `next_harvest_at = now() + 10min`을 UPDATE 하여 partial index에서 즉시 제외시킨다. 워커 크래시 시 lease가 만료되어 자동 회수.
- `Enqueue`는 두 frontier 테이블 중 어디에 쓸지 caller가 `QueueType`으로 결정한다(pioneer fanout 경로). 중복은 `INSERT ... ON CONFLICT (url_hash) DO NOTHING` 으로 처리하되, `harvester_frontier`는 `DECISIONS.md §8`의 UPSERT 규칙(`WHERE harvested_at IS NULL`)을 따른다. 본 change의 Enqueue는 **URL 문자열만** 받으므로 `snapshot_key` 등 구조화된 필드는 설정하지 않으며, 해당 필드의 초기/갱신은 후속 change(`harvester-scheduler-consumer`, `harvester-pin-document`)의 구조화된 enqueue 경로가 담당한다.
- `SetStatus(key, status, pinIDs)`는 fetch/harvest 결과를 frontier row에 반영하는 **완료 보고 채널**이다. `status` enum은 `"fetched"`, `"fetch_failed"`, `"harvested"`, `"harvest_failed"` 네 가지. `"harvested"` 호출 시에는 `harvester_frontier_pins` 테이블에 `(frontier_id, pin_id)` 매핑을 동일 트랜잭션 내에서 INSERT한다.
- `RecordFetchError(key, errorKind)` / `RecordHarvestError(key, errorKind)`는 **SetStatus와 분리된 별도 메서드**다. `errorKind` enum은 `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"` 네 가지. `"http_4xx"`는 즉시 dead(`error_count = 5`) 처리, 나머지는 `error_count++`와 `next_*_at` backoff 갱신(구체 공식은 `scheduler-retry-backoff`).
- **Consumer 호출 규약**: Pioneer/Harvester는 실패 시 `SetStatus(..., "*_failed", nil)` 와 `RecordFetchError`(또는 `RecordHarvestError`) 를 **둘 다** 호출한다. 성공 시에는 `SetStatus(..., "fetched"|"harvested", pinIDs)` 만 호출.
- **BREAKING (의사코드)**: `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue`는 본 change에서 `URLScheduler`로 rename되고, `Dequeue(string)` 시그니처는 `Dequeue(QueueType)`로 바뀐다. 호출부(Pioneer/Harvester 의사코드) 갱신 필요.
- 레거시 인메모리 큐 `apps/api/internal/bot/priority_queue.go`는 본 change의 후속 정리 단계에서 제거 대상으로 지정한다(실제 삭제는 호출부 마이그레이션 완료 후).

## Capabilities

### New Capabilities
<!-- scheduler capability는 이미 존재. 본 change는 추가 요구사항만 더한다. -->

### Modified Capabilities
- `scheduler`: 기존 scheduler capability에 URLScheduler claim API 요구사항을 추가한다 — interface 시그니처(Go 타입 shape는 design.md 참조), `QueueType` enum, Dequeue의 linearizability/block-on-empty/host throttle 통합, Enqueue의 upsert 의미, SetStatus의 status enum과 harvester_frontier_pins INSERT 책임, RecordFetchError/RecordHarvestError의 errorKind enum과 4xx 즉시 dead 규칙. 테이블 스키마(`scheduler-frontier-table`), backoff 공식(`scheduler-retry-backoff`), host token bucket 자체(`scheduler-host-token-bucket`)는 별도 change에서 다룬다.

## Impact

- **코드**: 새 패키지 `apps/api/internal/scheduler/` 에 `URLScheduler` interface와 Postgres 구현체를 추가. sqlc 쿼리(`enqueue_pioneer`, `enqueue_harvester`, `claim_pioneer`, `claim_harvester`, `set_status_fetched`, `set_status_fetch_failed`, `set_status_harvested`, `set_status_harvest_failed`, `insert_harvester_frontier_pins`, `record_fetch_error`, `record_harvest_error`) 신설.
- **DB**: 신규 마이그레이션 없음. `scheduler-frontier-table`이 만든 `pioneer_frontier`, `harvester_frontier`, `harvester_frontier_pins`와 partial index를 그대로 사용.
- **호출부**: Pioneer/Harvester는 본 change 범위에서 직접 교체하지 않고, 후속 change(`harvester-scheduler-consumer`, `pioneer-*`)에서 `URLScheduler`로 전환. 본 change는 contract 정의가 목적이며 단독 머지 가능.
- **운영**: `Dequeue` 폴링이 빈 큐/host throttle 상태에서 1초당 한 번 SELECT를 수행하므로, 워커 N개 × 1 QPS의 idle load가 발생. 부하 모니터링 필요.
- **문서**: `docs/architecture.md`의 bot 섹션에 URLScheduler interface 한 단락 추가, `apps/api/fuguebot_pseudo.go` 주석에 rename 사실 반영.
