## ADDED Requirements

### Requirement: URLScheduler interface 시그니처가 정의된다
시스템은 `URLScheduler`라는 이름의 Go interface를 제공해야 하며(SHALL), 다음 다섯 메서드를 정확히 가져야 한다(SHALL):
- `Enqueue(queueType QueueType, urls ...string) error` — `QueueType`으로 대상 frontier 테이블을 지정하여 URL을 등록한다.
- `Dequeue(queueType QueueType) (url string, err error)` — 주어진 큐 타입의 partial index에서 row 한 개를 claim하여 그 URL을 반환한다.
- `SetStatus(key string, status string, pinIDs []int64) error` — `key`(= `normalized_url`)로 식별되는 frontier row에 완료/실패 결과를 반영한다. `"harvested"` 호출 시 `pinIDs`를 `harvester_frontier_pins`에 기록한다.
- `RecordFetchError(key string, errorKind string) error` — Pioneer fetch 실패 시 `errorKind`에 따라 `fetch_error_count`와 `next_fetch_at`을 갱신한다.
- `RecordHarvestError(key string, errorKind string) error` — Harvester harvest 실패 시 `harvest_error_count`와 `next_harvest_at`을 갱신한다.

`QueueType`은 Go 상수 `QueuePioneer = "pioneer"`, `QueueHarvester = "harvester"` 두 값만 정의되어야 한다(SHALL).

`apps/api/fuguebot_pseudo.go`의 의사 타입 `URLPriorityQueue`는 본 interface로 대체되어야 한다(SHALL). "PriorityQueue"라는 이름의 타입을 새 코드에 도입해서는 안 된다(SHALL NOT).

#### Scenario: interface가 패키지에 노출된다
- **WHEN** 호출부가 scheduler 패키지를 import할 때
- **THEN** `URLScheduler` 라는 이름의 interface 타입과 `QueueType` enum이 export되어 있고, 위 다섯 메서드 시그니처가 정확히 일치한다.

#### Scenario: QueueType enum 값 제한
- **WHEN** scheduler 패키지의 상수를 확인할 때
- **THEN** `QueuePioneer`, `QueueHarvester` 두 값만 정의되어 있고, 세 번째 QueueType 값은 존재하지 않는다.

#### Scenario: 의사코드 타입과의 관계
- **WHEN** `apps/api/fuguebot_pseudo.go`를 확인할 때
- **THEN** `URLPriorityQueue`는 `URLScheduler`로 rename되어 있거나, 본 interface로 대체되었음을 가리키는 deprecation 주석이 달려 있다.

#### Scenario: PriorityQueue 명칭 금지
- **WHEN** 새로 추가되는 scheduler 관련 코드의 타입명을 확인할 때
- **THEN** "PriorityQueue"라는 이름의 타입은 새로 도입되지 않는다(레거시 `internal/bot/priority_queue.go`는 별도 정리 대상).

---

### Requirement: Dequeue는 linearizable하다
`URLScheduler.Dequeue`는 동일한 frontier row가 두 개 이상의 워커에 동시에 반환되지 않음을 보장해야 한다(SHALL). 구현은 Postgres `SELECT ... FOR UPDATE SKIP LOCKED` 패턴을 사용해야 하며(SHALL), 단순 `SELECT` 후 application 측 in-memory 락에만 의존해서는 안 된다(SHALL NOT).

#### Scenario: 두 워커 동시 dequeue 시 중복 없음
- **WHEN** 두 워커가 동일한 `QueueType`으로 동시에 `Dequeue`를 호출하고 frontier에 claim 가능한 row가 N개 있을 때
- **THEN** 각 워커는 서로 다른 row의 URL을 받으며, 동일 `url_hash`가 두 워커에 동시에 반환되지 않는다.

#### Scenario: claim 트랜잭션이 SKIP LOCKED를 사용
- **WHEN** Dequeue 구현의 SQL 쿼리를 확인할 때
- **THEN** `FOR UPDATE SKIP LOCKED` 절이 포함된 SELECT가 사용된다.

#### Scenario: 워커 죽음에 따른 자동 회수
- **WHEN** 한 워커가 `FOR UPDATE`로 row를 잠근 직후 in-flight marker를 set하기 전에 connection이 끊어질 때
- **THEN** Postgres가 락을 해제하여 다른 워커가 동일 row를 다시 claim할 수 있다.

---

### Requirement: Dequeue는 빈 큐/host throttle에서 block한다 (1초 고정 폴링)
`URLScheduler.Dequeue`는 claim 가능한 row가 없거나 host rate limiter가 모든 후보를 reject한 경우, 즉시 빈 문자열을 반환하거나 에러를 던져서는 안 된다(SHALL NOT). 대신, 1초 고정 간격의 폴링 루프로 대기하여 row가 claim 가능해지면 그 URL을 반환해야 한다(SHALL). 빈 큐와 host throttle로 인한 claim 실패는 **구분 없이 동일하게** 1초 sleep으로 처리되어야 한다(SHALL).

#### Scenario: 빈 frontier에서의 호출
- **WHEN** 대상 frontier 테이블에 claim 가능한 row가 0개인 상태에서 `Dequeue`가 호출될 때
- **THEN** 호출은 반환하지 않고 폴링을 시작한다.

#### Scenario: host throttle로 인한 block
- **WHEN** partial index 상위 후보 row들의 host에 대해 `HostRateLimiter.Allow(host)`가 모두 false를 반환할 때
- **THEN** 호출은 반환하지 않고 1초 sleep 후 재시도한다.

#### Scenario: 폴링 주기 1초 고정
- **WHEN** 빈 큐 또는 host throttle 상태에서 Dequeue가 폴링 중일 때
- **THEN** 두 시도 사이의 간격은 약 1초이며 (`time.Sleep(1 * time.Second)`), exponential backoff나 hot loop이 발생하지 않는다.

#### Scenario: enqueue 후 다음 폴링 사이클에서 wake-up
- **WHEN** Dequeue가 빈 큐에서 폴링 대기 중이고 다른 프로세스가 조건에 맞는 URL을 enqueue할 때
- **THEN** 늦어도 다음 폴링 사이클(약 1초 이내)에 해당 URL이 dequeue되어 반환된다.

---

### Requirement: Claim 프로토콜은 top N 후보 + HostRateLimiter.Allow 로 구성된다
`URLScheduler.Dequeue`의 내부 단일 시도는 다음 순서를 따라야 한다(SHALL):

1. partial index ORDER BY(`score DESC, next_*_at ASC`)로 상위 **N rows**를 `FOR UPDATE SKIP LOCKED`로 잠근다. N은 환경변수 `SCHEDULER_CLAIM_CANDIDATE_N`로 설정 가능하며 default 값은 1이어야 한다(SHALL).
2. 잠긴 각 row에 대해 `host` 컬럼 값으로 `HostRateLimiter.Allow(host)`를 순차 호출한다.
3. 처음 `true`를 반환한 row를 winner로 확정한다.
4. winner의 `next_fetch_at`(Pioneer) 또는 `next_harvest_at`(Harvester)을 `now() + interval '10 minutes'`로 UPDATE하여 in-flight marker로 사용한다. Lease timeout은 10분으로 **고정**되어야 한다(SHALL).
5. 트랜잭션을 COMMIT하고 winner의 URL을 반환한다.
6. 모든 후보가 false이면 트랜잭션을 ROLLBACK하고 "claim 실패"로 처리한다. Dequeue는 이어서 1초 sleep 후 재시도한다.

별도 `claimed_at` / `in_flight` 컬럼을 도입해서는 안 된다(SHALL NOT). in-flight marker는 `next_fetch_at` / `next_harvest_at` 재활용만 사용한다.

#### Scenario: top N 후보 잠금
- **WHEN** `SCHEDULER_CLAIM_CANDIDATE_N=3`으로 설정되고 partial index에 3개 이상의 row가 있을 때
- **THEN** claim 쿼리는 `LIMIT 3 FOR UPDATE SKIP LOCKED`로 3 row를 잠근다.

#### Scenario: 첫 통과 row claim
- **WHEN** N=3이고 첫 번째 row host는 throttle, 두 번째 row host는 허용일 때
- **THEN** 두 번째 row가 winner로 claim되고, 세 번째 row는 `HostRateLimiter.Allow` 호출 없이 (또는 호출 여부와 무관하게) claim되지 않는다.

#### Scenario: lease 10분 고정
- **WHEN** Pioneer Dequeue가 row를 claim할 때
- **THEN** 해당 row의 `next_fetch_at`은 정확히 `now() + 10 minutes`로 갱신되며, 이 값은 환경변수로 조정되지 않는다.

#### Scenario: lease 만료 시 자동 재claim
- **WHEN** 한 워커가 row를 claim한 뒤 SetStatus/RecordFetchError 호출 없이 10분 이상 경과할 때
- **THEN** `next_fetch_at <= now()` 조건이 다시 참이 되어 다른 워커가 동일 row를 claim할 수 있다.

#### Scenario: 별도 in-flight 컬럼 미도입
- **WHEN** scheduler 구현의 스키마 의존성을 확인할 때
- **THEN** `pioneer_frontier` / `harvester_frontier`에 `claimed_at`, `in_flight`, `lease_expires_at` 류의 컬럼이 **추가되지 않는다**.

---

### Requirement: Dequeue는 QueueType 기반으로 대상 테이블을 결정한다
`URLScheduler.Dequeue`의 인자 타입은 `QueueType` enum이어야 하며(SHALL), `QueuePioneer`는 `pioneer_frontier`에서, `QueueHarvester`는 `harvester_frontier`에서 claim을 수행해야 한다(SHALL). 동일 스케줄러 인스턴스가 두 큐 타입에 대해 독립적으로 호출될 수 있어야 한다(SHALL).

WHERE 절 조립용 별도 추상(`queryCondition` 타입, 쿼리 빌더 closure 등)을 도입해서는 안 된다(SHALL NOT).

#### Scenario: Pioneer claim 조건
- **WHEN** `Dequeue(QueuePioneer)`가 호출될 때
- **THEN** 내부 쿼리는 `pioneer_frontier`에서 `fetch_error_count < 5 AND next_fetch_at <= now()`를 만족하는 row를 대상으로 한다.

#### Scenario: Harvester claim 조건
- **WHEN** `Dequeue(QueueHarvester)`가 호출될 때
- **THEN** 내부 쿼리는 `harvester_frontier`에서 `harvested_at IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now()`를 만족하는 row를 대상으로 한다.

#### Scenario: partial index 매칭
- **WHEN** 위 두 쿼리의 실행 계획을 EXPLAIN으로 분석할 때
- **THEN** 각각 `scheduler-frontier-table` change가 정의한 pioneer/harvester partial index를 사용한다.

#### Scenario: queryCondition 추상 부재
- **WHEN** scheduler 패키지의 코드를 확인할 때
- **THEN** `queryCondition` 또는 이에 준하는 WHERE 절 조립 추상 타입이 정의되어 있지 않다.

---

### Requirement: Enqueue는 url_hash 기준 upsert로 동작한다
`URLScheduler.Enqueue`는 동일 `url_hash`로 여러 번 호출되어도 멱등적으로 동작해야 하며(SHALL), DB unique constraint violation을 호출자에게 노출해서는 안 된다(SHALL NOT).

- `QueuePioneer` Enqueue는 `INSERT INTO pioneer_frontier ... ON CONFLICT (url_hash) DO NOTHING` 형태여야 한다(SHALL).
- `QueueHarvester` Enqueue는 `DECISIONS.md §8`의 UPSERT 규칙을 따라야 한다(SHALL): `ON CONFLICT (url_hash) DO UPDATE SET snapshot_key = EXCLUDED.snapshot_key, next_harvest_at = now(), harvest_error_count = 0 WHERE harvester_frontier.harvested_at IS NULL`.

#### Scenario: 동일 URL 중복 enqueue가 멱등 (pioneer)
- **WHEN** `Enqueue(QueuePioneer, url)`가 동일 URL로 두 번 연속 호출될 때
- **THEN** 첫 호출은 row를 생성하고, 두 번째 호출은 에러 없이 반환되며 `pioneer_frontier`에 정확히 1개의 row만 존재한다.

#### Scenario: Harvester UPSERT — 이미 harvest된 URL 재enqueue
- **WHEN** `harvested_at IS NOT NULL`인 row가 존재하는 상태에서 `Enqueue(QueueHarvester, url)`가 다시 호출될 때
- **THEN** 해당 row는 갱신되지 않고 no-op으로 처리된다(재harvest 금지).

#### Scenario: Harvester UPSERT — 아직 harvest되지 않은 URL 재enqueue
- **WHEN** `harvested_at IS NULL`인 row가 존재하는 상태에서 `Enqueue(QueueHarvester, url)`가 다시 호출될 때
- **THEN** 해당 row의 `snapshot_key`, `next_harvest_at`, `harvest_error_count`가 갱신된다.

#### Scenario: 가변인자 batch enqueue
- **WHEN** Enqueue가 여러 URL을 동시에 받을 때 (`Enqueue(QueuePioneer, u1, u2, u3)`)
- **THEN** 모두 한 트랜잭션 또는 한 batch 안에서 upsert되며, 일부가 conflict로 무시되어도 나머지는 성공적으로 등록된다.

#### Scenario: unique violation 미노출
- **WHEN** 호출자가 Enqueue를 사용할 때
- **THEN** Postgres unique constraint violation 에러는 호출자에게 노출되지 않는다.

---

### Requirement: SetStatus는 status enum 4종을 처리하고 harvested 시 pin 매핑을 저장한다
`URLScheduler.SetStatus(key, status, pinIDs)`는 `key`(= `normalized_url`)에 해당하는 frontier row를 갱신해야 하며(SHALL), 다음 네 개의 status 값만 허용한다(SHALL):

- `"fetched"`: `pioneer_frontier.last_fetched_at = now()` 갱신 및 `next_fetch_at = now() + 365 days` 로 재크롤 시점 예약. `pinIDs`는 무시된다.
- `"fetch_failed"`: Pioneer 실패 마킹. `last_updated_at` 갱신. `fetch_error_count` 증가와 `next_fetch_at` backoff는 **SetStatus의 책임이 아니다** (RecordFetchError 경로).
- `"harvested"`: `harvester_frontier.harvested_at = now()` 갱신과 함께, **동일 DB 트랜잭션** 내에서 `pinIDs` 각 요소에 대해 `INSERT INTO harvester_frontier_pins (frontier_id, pin_id)`를 수행해야 한다(SHALL). `pinIDs`가 비어 있으면(길이 0) INSERT는 스킵한다.
- `"harvest_failed"`: Harvester 실패 마킹. `last_updated_at` 갱신. `harvest_error_count` / `next_harvest_at` 갱신은 RecordHarvestError 책임.

위 네 값 이외의 status 문자열은 에러로 처리되어야 한다(SHALL).

본 change는 `next_fetch_at` / `next_harvest_at` 의 backoff 공식을 정의하지 않으며(NON-NORMATIVE), 그 책임은 `scheduler-retry-backoff` change에 있다.

#### Scenario: fetched status 처리
- **WHEN** Pioneer consumer가 `SetStatus("https://example.com/x", "fetched", nil)` 를 호출할 때
- **THEN** 해당 row의 `last_fetched_at`이 호출 시각으로 갱신되고 `next_fetch_at`은 약 1년 뒤로 예약되며, Pioneer claim partial index에서 해당 row가 제거된다.

#### Scenario: fetch_failed status 처리 (SetStatus 단독)
- **WHEN** Pioneer consumer가 `SetStatus(..., "fetch_failed", nil)` 를 호출했지만 RecordFetchError는 아직 호출하지 않았을 때
- **THEN** `fetch_error_count`는 증가하지 않는다(RecordFetchError의 책임). SetStatus는 마킹 의미만 갖는다.

#### Scenario: harvested status 처리 — pin 매핑 INSERT
- **WHEN** Harvester consumer가 `SetStatus("https://example.com/x", "harvested", []int64{42, 43})` 를 호출할 때
- **THEN** 해당 `harvester_frontier` row의 `harvested_at`이 갱신되고, `harvester_frontier_pins`에 `(frontier_id, 42)` 및 `(frontier_id, 43)` 행이 동일 트랜잭션에서 INSERT된다.

#### Scenario: harvested status 원자성
- **WHEN** `harvester_frontier` UPDATE는 성공했으나 `harvester_frontier_pins` INSERT 중 하나가 실패할 때
- **THEN** 트랜잭션 전체가 롤백되어 `harvested_at`도 갱신되지 않는다.

#### Scenario: harvested status — 빈 pinIDs
- **WHEN** Harvester consumer가 `SetStatus(..., "harvested", nil)` 또는 `[]int64{}`로 호출할 때
- **THEN** `harvested_at`만 갱신되고 `harvester_frontier_pins` INSERT는 실행되지 않는다.

#### Scenario: harvest_failed status 처리
- **WHEN** Harvester consumer가 `SetStatus(..., "harvest_failed", nil)` 를 호출할 때
- **THEN** `harvester_frontier.last_updated_at`은 갱신되지만, `harvest_error_count`와 `next_harvest_at`은 SetStatus에서 변경되지 않는다.

#### Scenario: 알 수 없는 status
- **WHEN** SetStatus가 네 enum 이외의 status 문자열로 호출될 때
- **THEN** 에러가 반환되며 DB는 변경되지 않는다.

#### Scenario: 알 수 없는 key
- **WHEN** SetStatus가 frontier에 존재하지 않는 `key`로 호출될 때
- **THEN** 새 row를 만들지 않고 warn 로그만 기록한다(panic 금지).

---

### Requirement: RecordFetchError/RecordHarvestError는 errorKind enum 4종을 처리한다
`URLScheduler.RecordFetchError(key, errorKind)`와 `RecordHarvestError(key, errorKind)`는 각각 Pioneer/Harvester 경로의 실패 집계를 담당해야 하며(SHALL), SetStatus와는 **별도 메서드**로 유지되어야 한다(SHALL).

허용되는 `errorKind` 값은 네 가지다(SHALL):
- `"http_4xx"`: 해당 row의 `fetch_error_count`(또는 `harvest_error_count`)를 **즉시 5로 설정**하여 dead 상태로 만든다. backoff 공식을 적용하지 않는다(SHALL).
- `"http_5xx"`, `"network"`, `"timeout"`: `error_count`를 1 증가시키고, `next_fetch_at`(또는 `next_harvest_at`)을 `scheduler-retry-backoff` change의 backoff 공식으로 갱신한다.

위 네 값 이외의 errorKind는 에러로 처리되어야 한다(SHALL).

**Consumer 호출 규약**: Pioneer/Harvester는 실패 시 `SetStatus(key, "fetch_failed"|"harvest_failed", nil)` 와 해당 `RecordFetchError`/`RecordHarvestError`를 **둘 다** 호출해야 한다(SHALL). 성공 시에는 `SetStatus` 만 호출하고 RecordXxxError는 호출하지 않는다.

#### Scenario: http_4xx 즉시 dead
- **WHEN** `RecordFetchError(key, "http_4xx")`가 호출될 때
- **THEN** 해당 row의 `fetch_error_count`가 즉시 5로 설정되고 partial index에서 제외된다. backoff 공식은 적용되지 않는다.

#### Scenario: http_5xx / network / timeout 증가
- **WHEN** `RecordFetchError(key, "http_5xx")`(또는 `"network"`, `"timeout"`)가 호출될 때
- **THEN** 해당 row의 `fetch_error_count`가 1 증가하고 `next_fetch_at`이 backoff 공식으로 갱신된다.

#### Scenario: Harvester 경로 동일
- **WHEN** `RecordHarvestError(key, "http_4xx")`가 호출될 때
- **THEN** 해당 row의 `harvest_error_count`가 즉시 5로 설정된다.

#### Scenario: SetStatus와의 분리
- **WHEN** Consumer가 실패 시 `SetStatus(key, "fetch_failed", nil)` 만 호출하고 RecordFetchError는 호출하지 않을 때
- **THEN** `fetch_error_count`는 증가하지 않으며, 이는 consumer의 규약 위반이다(본 스펙은 consumer가 둘 다 호출할 것을 요구).

#### Scenario: 알 수 없는 errorKind
- **WHEN** RecordFetchError/RecordHarvestError가 네 enum 이외의 errorKind로 호출될 때
- **THEN** 에러가 반환되며 DB는 변경되지 않는다.

#### Scenario: 알 수 없는 key
- **WHEN** RecordFetchError/RecordHarvestError가 frontier에 존재하지 않는 `key`로 호출될 때
- **THEN** 새 row를 만들지 않고 warn 로그만 기록한다(panic 금지).

---

### Requirement: 호출부는 본 change에서 교체되지 않는다
본 change는 `URLScheduler` interface와 Postgres 구현체, 단위/통합 테스트만 포함해야 하며(SHALL), Pioneer/Harvester 실제 호출부 코드(예: `internal/bot/pioneer/*.go`, `internal/bot/harvester/*.go`)를 본 change에서 교체해서는 안 된다(SHALL NOT). 호출부 마이그레이션은 후속 change(`harvester-scheduler-consumer` 등)에서 수행된다.

#### Scenario: 호출부 미수정
- **WHEN** 본 change의 diff를 확인할 때
- **THEN** Pioneer/Harvester worker 진입점(`Run()` 등)의 큐 사용 코드는 변경되지 않는다.

#### Scenario: 레거시 큐 보존
- **WHEN** 본 change 머지 후 `apps/api/internal/bot/priority_queue.go`를 확인할 때
- **THEN** 파일은 여전히 존재하며, 본 change에서 삭제되지 않는다(후속 정리 change에서 제거).
