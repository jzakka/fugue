## ADDED Requirements

### Requirement: URLScheduler interface 시그니처가 정의된다
시스템은 `URLScheduler`라는 이름의 Go interface를 제공해야 하며(SHALL), 다음 다섯 메서드를 **최소한 제공해야 한다**(SHALL). 후속 change가 추가 메서드(예: context.Context를 받는 overload)를 더하는 것은 허용된다:
- `Enqueue(queueType QueueType, urls ...string) error` — `QueueType`으로 대상 frontier 테이블을 지정하여 URL을 등록한다.
- `Dequeue(queueType QueueType) (url string, err error)` — 주어진 큐 타입의 partial index에서 row 한 개를 claim하여 그 URL을 반환한다.
- `SetStatus(key string, status string, pinIDs []uuid.UUID) error` — `key`로 식별되는 frontier row에 완료/실패 결과를 반영한다. `key`는 이전 `Dequeue`가 반환했거나 이전 `Enqueue` 호출에 전달된 URL 문자열과 동치여야 하며, 내부적으로는 해당 URL의 정규화 결과로부터 유도된 `url_hash`로 lookup된다. `"harvested"` 호출 시 `pinIDs`를 `harvester_frontier_pins`에 기록한다.
- `RecordFetchError(key string, errorKind string) error` — Pioneer fetch 실패 시 `errorKind`에 따라 `fetch_error_count`와 `next_fetch_at`을 갱신한다. `key` 규약은 SetStatus와 동일.
- `RecordHarvestError(key string, errorKind string) error` — Harvester harvest 실패 시 `harvest_error_count`와 `next_harvest_at`을 갱신한다. `key` 규약은 SetStatus와 동일.

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
`URLScheduler.Dequeue`는 동일한 frontier row가 두 개 이상의 워커에 동시에 반환되지 않음을 보장해야 한다(SHALL). 애플리케이션 레벨 인메모리 락만으로 이 보장을 대체해서는 안 되며(SHALL NOT), DB가 제공하는 행 단위 락을 활용해야 한다(SHALL). 구체적인 SQL 패턴(`SELECT ... FOR UPDATE SKIP LOCKED`)은 `design.md`에 정의한다.

#### Scenario: 두 워커 동시 dequeue 시 중복 없음
- **WHEN** 두 워커가 동일한 `QueueType`으로 동시에 `Dequeue`를 호출하고 frontier에 claim 가능한 row가 N개 있을 때
- **THEN** 각 워커는 서로 다른 row의 URL을 받으며, 동일 `url_hash`가 두 워커에 동시에 반환되지 않는다.

#### Scenario: 워커 죽음에 따른 자동 회수
- **WHEN** 한 워커가 `FOR UPDATE`로 row를 잠근 직후 in-flight marker를 set하기 전에 connection이 끊어질 때
- **THEN** Postgres가 락을 해제하여 다른 워커가 동일 row를 다시 claim할 수 있다.

#### Scenario: claim SELECT와 in-flight mark UPDATE가 동일 트랜잭션에서 수행
- **WHEN** Dequeue가 row를 잠그는 SELECT와 `next_*_at = now() + 10min` UPDATE를 실행할 때
- **THEN** 두 쿼리는 동일한 Postgres 트랜잭션 안에서 실행되며, 그 사이에 다른 워커가 동일 row를 다시 claim하거나 읽어들일 수 없다.

---

### Requirement: Dequeue는 빈 큐/host throttle 시 block-on-empty로 대기한다
`URLScheduler.Dequeue`는 claim 가능한 row가 없거나 host rate limiter가 모든 후보를 reject한 경우, 즉시 빈 문자열을 반환하거나 에러를 던져서는 안 된다(SHALL NOT). 대신, 약 1초 고정 간격의 폴링 루프로 대기하여 row가 claim 가능해지면 그 URL을 반환해야 한다(SHALL). 빈 큐와 host throttle로 인한 claim 실패는 **구분 없이 동일하게** 약 1초의 대기 후 재시도로 처리되어야 한다(SHALL). 구체적 sleep 구현(`time.Sleep`)은 `design.md`에 정의한다.

#### Scenario: 빈 frontier에서의 호출
- **WHEN** 대상 frontier 테이블에 claim 가능한 row가 0개인 상태에서 `Dequeue`가 호출될 때
- **THEN** 호출은 반환하지 않고 폴링을 시작한다.

#### Scenario: host throttle로 인한 block
- **WHEN** partial index 상위 후보 row들의 host에 대해 `HostRateLimiter.Allow(host)`가 모두 false를 반환할 때
- **THEN** 호출은 반환하지 않고 1초 sleep 후 재시도한다.

#### Scenario: 폴링 주기 1초 고정
- **WHEN** 빈 큐 또는 host throttle 상태에서 Dequeue가 폴링 중일 때
- **THEN** 두 시도 사이의 간격은 약 1초이며, exponential backoff나 hot loop이 발생하지 않는다.

#### Scenario: enqueue 후 다음 폴링 사이클에서 wake-up
- **WHEN** Dequeue가 빈 큐에서 폴링 대기 중이고 다른 프로세스가 조건에 맞는 URL을 enqueue할 때
- **THEN** 늦어도 다음 폴링 사이클(최대 약 2초 이내: 현재 sleep 잔여 + 다음 시도)에 해당 URL이 dequeue되어 반환된다.

---

### Requirement: Claim은 상위 후보군 중 호스트 토큰 버킷이 허용한 첫 row를 선택한다
`URLScheduler.Dequeue`의 내부 단일 시도는 다음 순서를 따라야 한다(SHALL):

1. partial index ORDER BY(`score DESC, next_*_at ASC`)로 상위 **N rows**를 DB 행 단위 락으로 잠근다. N은 설정 가능하며 default 값은 1이어야 한다(SHALL).
2. 잠긴 각 row에 대해 `host` 컬럼 값으로 `HostRateLimiter.Allow(host)`를 순차 호출한다.
3. 처음 `true`를 반환한 row를 winner로 확정한다.
4. winner의 `next_fetch_at`(Pioneer) 또는 `next_harvest_at`(Harvester)을 claim 시각 + 10분으로 UPDATE하여 in-flight marker로 사용한다. Lease timeout은 base scheduler spec과 동일하게 **10분**이다(SHALL).
5. 트랜잭션을 COMMIT하고 winner의 URL을 반환한다.
6. 모든 후보가 false이면 트랜잭션을 ROLLBACK하고 "claim 실패"로 처리한다. Dequeue는 이어서 약 1초 sleep 후 재시도한다.

in-flight 상태를 저장하기 위한 **새 컬럼을 `pioneer_frontier` / `harvester_frontier`에 추가해서는 안 된다**(SHALL NOT). in-flight 표시는 `next_fetch_at` / `next_harvest_at` 컬럼을 재활용하여만 구현한다. 구체적인 SQL 패턴(`FOR UPDATE SKIP LOCKED`, `interval '10 minutes'`, 환경변수명, 후보 N 설정 방법)은 `design.md`에 정의한다.

#### Scenario: top N 후보 잠금
- **WHEN** 후보 N이 3으로 설정되고 partial index에 3개 이상의 row가 있을 때
- **THEN** claim 시도는 상위 3개의 row를 동시에 DB 레벨에서 잠근다.

#### Scenario: 첫 통과 row claim
- **WHEN** N=3이고 첫 번째 row host는 throttle, 두 번째 row host는 허용일 때
- **THEN** 두 번째 row가 winner로 claim되고, 세 번째 row는 claim되지 않는다.

#### Scenario: lease 만료 시 자동 재claim
- **WHEN** 한 워커가 row를 claim한 뒤 SetStatus/RecordFetchError 호출 없이 10분 이상 경과할 때
- **THEN** `next_fetch_at <= now()` 조건이 다시 참이 되어 다른 워커가 동일 row를 claim할 수 있다.

#### Scenario: 별도 in-flight 컬럼 미도입
- **WHEN** 본 change가 도입하는 마이그레이션 diff를 확인할 때
- **THEN** `pioneer_frontier` / `harvester_frontier`에 in-flight 상태 추적용 새 컬럼이 추가되지 않는다.

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

- `QueuePioneer` Enqueue는 이미 존재하는 `url_hash`에 대해 **no-op으로 수행되어야 한다**(SHALL). 기존 row의 field는 변경하지 않는다.
- `QueueHarvester` Enqueue는 `DECISIONS.md §8`의 UPSERT 규칙을 따라야 한다(SHALL): 이미 `harvested_at IS NOT NULL`인 row는 no-op, `harvested_at IS NULL`인 row는 재enqueue 의도(next_harvest_at / harvest_error_count 초기화)를 반영하여 갱신된다.

구체적인 SQL 패턴(`INSERT ... ON CONFLICT ...` 절)은 `design.md`에 정의한다.

#### Scenario: 동일 URL 중복 enqueue가 멱등 (pioneer)
- **WHEN** `Enqueue(QueuePioneer, url)`가 동일 URL로 두 번 연속 호출될 때
- **THEN** 첫 호출은 row를 생성하고, 두 번째 호출은 에러 없이 반환되며 `pioneer_frontier`에 정확히 1개의 row만 존재한다.

#### Scenario: Harvester UPSERT — 이미 harvest된 URL 재enqueue
- **WHEN** `harvested_at IS NOT NULL`인 row가 존재하는 상태에서 `Enqueue(QueueHarvester, url)`가 다시 호출될 때
- **THEN** 해당 row는 갱신되지 않고 no-op으로 처리된다(재harvest 금지).

#### Scenario: Harvester UPSERT — 아직 harvest되지 않은 URL 재enqueue
- **WHEN** `harvested_at IS NULL`인 row가 존재하는 상태에서 `Enqueue(QueueHarvester, url)`가 다시 호출될 때
- **THEN** 해당 row의 `next_harvest_at` 및 `harvest_error_count`가 갱신된다. `snapshot_key`는 본 change의 Enqueue 경로에서 건드리지 않는다 (초기/갱신은 후속 change 책임).

#### Scenario: 가변인자 batch enqueue
- **WHEN** Enqueue가 여러 URL을 동시에 받을 때 (`Enqueue(QueuePioneer, u1, u2, u3)`)
- **THEN** 모두 한 트랜잭션 또는 한 batch 안에서 upsert되며, 일부가 conflict로 무시되어도 나머지는 성공적으로 등록된다.

#### Scenario: URL-only Enqueue의 NOT NULL 컬럼 기본값
- **WHEN** `Enqueue`가 URL만 받아 pioneer_frontier에 새 row를 생성할 때
- **THEN** `depth`는 0, `score`는 0.0으로 기록된다. BFS depth 전파와 score 계산은 본 change 범위가 아니며, 후속 change의 구조화된 enqueue 경로에서 다룬다(NON-NORMATIVE reference: design.md Decision 6).

#### Scenario: unique violation 미노출
- **WHEN** 호출자가 Enqueue를 사용할 때
- **THEN** Postgres unique constraint violation 에러는 호출자에게 노출되지 않는다.

---

### Requirement: SetStatus는 status enum 4종을 처리하고 harvested 시 pin 매핑을 저장한다
`URLScheduler.SetStatus(key, status, pinIDs)`는 `key`(이전 Enqueue/Dequeue에서 사용된 URL 문자열; 구현체가 정규화 후 `url_hash`로 lookup)에 해당하는 frontier row를 갱신해야 하며(SHALL), 다음 네 개의 status 값만 허용한다(SHALL):

- `"fetched"`: `pioneer_frontier.last_fetched_at = now()` 갱신 및 `next_fetch_at = now() + 365 days` 로 재크롤 시점 예약. 이전 fetch 시도에서 누적되었을 수 있는 `fetch_error_count`는 **0으로 리셋**되어야 한다(SHALL). `pinIDs`는 무시된다.
- `"fetch_failed"`: Pioneer 실패 마킹. `last_updated_at` 갱신. `fetch_error_count` 증가와 `next_fetch_at` backoff는 **SetStatus의 책임이 아니다** (RecordFetchError 경로).
- `"harvested"`: `harvester_frontier.harvested_at = now()` 갱신과 함께, **동일 DB 트랜잭션** 내에서 `pinIDs` 각 요소에 대해 `INSERT INTO harvester_frontier_pins (frontier_id, pin_id)`를 수행해야 한다(SHALL). 이전 harvest 시도에서 누적된 `harvest_error_count`는 **0으로 리셋**되어야 한다(SHALL). `pinIDs`가 비어 있으면(길이 0) INSERT는 스킵한다.
- `"harvest_failed"`: Harvester 실패 마킹. `last_updated_at` 갱신. `harvest_error_count` / `next_harvest_at` 갱신은 RecordHarvestError 책임.

위 네 값 이외의 status 문자열은 에러로 처리되어야 한다(SHALL).

본 change는 `next_fetch_at` / `next_harvest_at` 의 backoff 공식을 정의하지 않으며(NON-NORMATIVE), 그 책임은 `scheduler-retry-backoff` change에 있다.

#### Scenario: fetched status 처리
- **WHEN** Pioneer consumer가 `SetStatus("https://example.com/x", "fetched", nil)` 를 호출할 때
- **THEN** 해당 row의 `last_fetched_at`이 호출 시각으로 갱신되고 `next_fetch_at`은 약 1년 뒤로 예약되며, `fetch_error_count`가 0으로 리셋되고, Pioneer claim partial index에서 해당 row가 제거된다.

#### Scenario: fetched status — error_count 리셋
- **WHEN** `fetch_error_count = 3` 인 row에 대해 `SetStatus(key, "fetched", nil)` 이 호출될 때
- **THEN** 호출 후 `fetch_error_count = 0` 이며, 해당 URL이 다시 enqueue되어 실패할 경우 backoff는 첫 실패 수준에서 재시작한다.

#### Scenario: fetch_failed status 처리 (SetStatus 단독)
- **WHEN** Pioneer consumer가 `SetStatus(..., "fetch_failed", nil)` 를 호출했지만 RecordFetchError는 아직 호출하지 않았을 때
- **THEN** `fetch_error_count`는 증가하지 않는다(RecordFetchError의 책임). SetStatus는 마킹 의미만 갖는다.

#### Scenario: harvested status 처리 — pin 매핑 INSERT
- **WHEN** Harvester consumer가 `SetStatus("https://example.com/x", "harvested", []uuid.UUID{pinA, pinB})` 를 호출할 때 (pinA, pinB는 기존에 생성된 pin UUID)
- **THEN** 해당 `harvester_frontier` row의 `harvested_at`이 갱신되고 `harvest_error_count`가 0으로 리셋되며, `harvester_frontier_pins`에 `(frontier_id, pinA)` 및 `(frontier_id, pinB)` 행이 동일 트랜잭션에서 INSERT된다.

#### Scenario: harvested status 원자성
- **WHEN** `harvester_frontier` UPDATE는 성공했으나 `harvester_frontier_pins` INSERT 중 하나가 실패할 때
- **THEN** 트랜잭션 전체가 롤백되어 `harvested_at`도 갱신되지 않는다.

#### Scenario: harvested status — 빈 pinIDs
- **WHEN** Harvester consumer가 `SetStatus(..., "harvested", nil)` 또는 `[]uuid.UUID{}`로 호출할 때
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

예외: `apps/api/fuguebot_pseudo.go`는 실제 실행 경로가 아닌 의사코드 파일이므로, 본 change는 `URLPriorityQueue` 타입/호출 부분에 **deprecation 주석만** 추가할 수 있다(SHALL). 타입 rename, 시그니처 변경, 호출 치환은 하지 않는다(SHALL NOT).

#### Scenario: 호출부 미수정
- **WHEN** 본 change의 diff를 확인할 때
- **THEN** Pioneer/Harvester worker 진입점(`Run()` 등)의 큐 사용 코드는 변경되지 않는다.

#### Scenario: fuguebot_pseudo.go는 주석만 추가
- **WHEN** 본 change의 `apps/api/fuguebot_pseudo.go` diff를 확인할 때
- **THEN** `URLPriorityQueue` 타입 정의와 호출부 코드 자체는 유지되며, 추가된 변경은 "후속 change에서 `URLScheduler`로 교체 예정" 취지의 주석뿐이다.

#### Scenario: 레거시 큐 보존
- **WHEN** 본 change 머지 후 `apps/api/internal/bot/priority_queue.go`를 확인할 때
- **THEN** 파일은 여전히 존재하며, 본 change에서 삭제되지 않는다(후속 정리 change에서 제거).
