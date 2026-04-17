## ADDED Requirements

### Requirement: URLScheduler interface 시그니처가 정의된다
시스템은 `URLScheduler`라는 이름의 Go interface를 제공해야 하며(SHALL), 다음 세 메서드를 정확히 가져야 한다(SHALL):
- `Enqueue(urls ...string)` — 가변인자로 URL을 받아 frontier에 등록한다.
- `Dequeue(cond queryCondition) string` — 주어진 쿼리 조건을 만족하는 row 한 개를 claim하여 그 URL을 반환한다.
- `SetStatus(key string, status string)` — `key`(= `normalized_url`)로 식별되는 frontier row에 처리 결과를 반영한다.

`apps/api/fuguebot_pseudo.go`의 의사 타입 `URLPriorityQueue`는 본 interface로 대체되어야 한다(SHALL). "PriorityQueue"라는 이름의 타입을 새 코드에 도입해서는 안 된다(SHALL NOT).

#### Scenario: interface가 패키지에 노출된다
- **WHEN** 호출부가 scheduler 패키지를 import할 때
- **THEN** `URLScheduler` 라는 이름의 interface 타입이 export되어 있고, 위 세 메서드 시그니처가 정확히 일치한다.

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
- **WHEN** 두 워커가 동일한 `queryCondition`으로 동시에 `Dequeue`를 호출하고 frontier에 claim 가능한 row가 N개 있을 때
- **THEN** 각 워커는 서로 다른 row의 URL을 받으며, 동일 `normalized_url`이 두 워커에 동시에 반환되지 않는다.

#### Scenario: claim 트랜잭션이 SKIP LOCKED를 사용
- **WHEN** Dequeue 구현의 SQL 쿼리를 확인할 때
- **THEN** `FOR UPDATE SKIP LOCKED` 절이 포함된 SELECT가 사용된다.

#### Scenario: 워커 죽음에 따른 자동 회수
- **WHEN** 한 워커가 `FOR UPDATE`로 row를 잠근 직후 in-flight marker를 set하기 전에 connection이 끊어질 때
- **THEN** Postgres가 락을 해제하여 다른 워커가 동일 row를 다시 claim할 수 있다.

---

### Requirement: Dequeue는 빈 큐에서 block한다 (busy-wait + 1초 sleep)
`URLScheduler.Dequeue`는 claim 가능한 row가 없으면 즉시 빈 문자열을 반환하거나 에러를 던져서는 안 된다(SHALL NOT). 대신, 1초 간격의 폴링 루프로 대기하여 row가 등장하면 그 URL을 반환해야 한다(SHALL).

#### Scenario: 빈 frontier에서의 호출
- **WHEN** frontier에 `queryCondition`을 만족하는 row가 0개인 상태에서 `Dequeue`가 호출될 때
- **THEN** 호출은 반환하지 않고 폴링을 시작한다.

#### Scenario: 폴링 주기 1초
- **WHEN** 빈 큐 상태에서 Dequeue가 폴링 중일 때
- **THEN** 두 SELECT 시도 사이의 간격은 약 1초이며 (`time.Sleep(1 * time.Second)`), 더 짧은 hot loop이 발생하지 않는다.

#### Scenario: enqueue 후 다음 폴링 사이클에서 wake-up
- **WHEN** Dequeue가 빈 큐에서 폴링 대기 중이고 다른 워커가 조건에 맞는 URL을 enqueue할 때
- **THEN** 늦어도 다음 폴링 사이클(약 1초 이내)에 해당 URL이 dequeue되어 반환된다.

---

### Requirement: Dequeue 인자는 status 문자열이 아니라 queryCondition이다
`URLScheduler.Dequeue`의 인자 타입은 단순 status enum이나 string이어서는 안 된다(SHALL NOT). 대신, claim SQL의 WHERE 절을 조립하는 **쿼리 조건 객체**(`queryCondition`)여야 하며(SHALL), 동일 스케줄러 인스턴스가 서로 다른 조건으로 호출되어 서로 다른 partial index를 사용할 수 있어야 한다(SHALL).

#### Scenario: Pioneer claim 조건
- **WHEN** Pioneer 워커가 Dequeue를 호출할 때
- **THEN** 전달된 `queryCondition`은 개념적으로 `last_fetched_at IS NULL AND fetch_error_count < 5 AND next_fetch_at <= now()` 와 동치인 SQL 조건을 생성한다.

#### Scenario: Harvester claim 조건
- **WHEN** Harvester 워커가 Dequeue를 호출할 때
- **THEN** 전달된 `queryCondition`은 개념적으로 `pin_id IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now()` 와 동치인 SQL 조건을 생성한다.

#### Scenario: Pioneer/Harvester partial index 매칭
- **WHEN** 위 두 조건의 SELECT를 EXPLAIN으로 분석할 때
- **THEN** Pioneer 조건은 `bot_frontier_pioneer_claimable_idx`를, Harvester 조건은 `bot_frontier_harvester_claimable_idx`를 사용한다.

#### Scenario: status enum 부재
- **WHEN** scheduler 패키지의 코드를 확인할 때
- **THEN** Pioneer/Harvester 분기를 위한 status enum 타입(`type Status string` 등)이 정의되어 있지 않으며, 대신 `queryCondition` 추상으로 분기된다.

---

### Requirement: Enqueue는 normalized_url 기준 upsert로 동작한다
`URLScheduler.Enqueue`는 동일 `normalized_url`을 여러 번 호출해도 멱등적으로 동작해야 하며(SHALL), DB unique constraint violation을 호출자에게 노출해서는 안 된다(SHALL NOT). 구현은 `INSERT ... ON CONFLICT (normalized_url) DO NOTHING` (또는 명시적으로 정의된 score 갱신 정책) 형태여야 한다(SHALL).

#### Scenario: 동일 URL 중복 enqueue가 멱등
- **WHEN** Enqueue가 동일 URL을 두 번 연속 호출될 때
- **THEN** 첫 호출은 row를 생성하고, 두 번째 호출은 에러 없이 반환되며 frontier에는 정확히 1개의 row만 존재한다.

#### Scenario: 가변인자 batch enqueue
- **WHEN** Enqueue가 여러 URL을 동시에 받을 때 (`Enqueue(u1, u2, u3)`)
- **THEN** 모두 한 트랜잭션 또는 한 batch 안에서 upsert되며, 일부가 conflict로 무시되어도 나머지는 성공적으로 등록된다.

#### Scenario: unique violation 미노출
- **WHEN** 호출자가 Enqueue를 사용할 때
- **THEN** Postgres unique constraint violation 에러는 호출자에게 노출되지 않는다(silent하게 처리되거나, 호출자가 신경 쓰지 않아도 되는 형태로 추상화된다).

---

### Requirement: SetStatus는 fetch/harvest 결과를 frontier row에 반영한다
`URLScheduler.SetStatus(key, status)`는 `key`(= `normalized_url`)에 해당하는 frontier row를 갱신하는 채널이어야 하며(SHALL), 본 change는 다음 status 문자열을 표준화한다(SHALL):
- `"fetched"`: `last_fetched_at = now()` 갱신.
- `"fetch_failed"`: `fetch_error_count = fetch_error_count + 1` 갱신.
- `"harvested:<pin_id>"`: `pin_id`를 주어진 값으로 갱신.
- `"harvest_failed"`: `harvest_error_count = harvest_error_count + 1` 갱신.

본 change는 `next_fetch_at` / `next_harvest_at` 의 구체 산정 공식을 정의하지 않으며(NON-NORMATIVE), 그 책임은 `scheduler-retry-backoff` change에 있다.

#### Scenario: fetched status 처리
- **WHEN** Pioneer가 `SetStatus("https://example.com/x", "fetched")` 를 호출할 때
- **THEN** 해당 row의 `last_fetched_at`이 호출 시각으로 갱신되고, Pioneer claim partial index에서 해당 row가 제거된다.

#### Scenario: fetch_failed status 처리
- **WHEN** Pioneer가 `SetStatus("https://example.com/x", "fetch_failed")` 를 호출할 때
- **THEN** 해당 row의 `fetch_error_count`가 1 증가하고, 5 미만인 동안에는 Pioneer claim partial index에 남는다.

#### Scenario: harvested status 처리
- **WHEN** Harvester가 `SetStatus("https://example.com/x", "harvested:42")` 를 호출할 때
- **THEN** 해당 row의 `pin_id`가 42로 갱신되고, Harvester claim partial index에서 해당 row가 제거된다.

#### Scenario: harvest_failed status 처리
- **WHEN** Harvester가 `SetStatus("https://example.com/x", "harvest_failed")` 를 호출할 때
- **THEN** 해당 row의 `harvest_error_count`가 1 증가한다.

#### Scenario: 알 수 없는 key는 무시되거나 에러 로깅
- **WHEN** SetStatus가 frontier에 존재하지 않는 `key`로 호출될 때
- **THEN** 새 row를 만들지 않고 noop으로 처리되거나 명확한 에러가 로깅된다(panic 금지).

---

### Requirement: 호출부는 본 change에서 교체되지 않는다
본 change는 `URLScheduler` interface와 Postgres 구현체, 단위/통합 테스트만 포함해야 하며(SHALL), Pioneer/Harvester 실제 호출부 코드(예: `internal/bot/pioneer/*.go`, `internal/bot/harvester/*.go`)를 본 change에서 교체해서는 안 된다(SHALL NOT). 호출부 마이그레이션은 후속 change(`harvester-scheduler-consumer` 등)에서 수행된다.

#### Scenario: 호출부 미수정
- **WHEN** 본 change의 diff를 확인할 때
- **THEN** Pioneer/Harvester worker 진입점(`Run()` 등)의 큐 사용 코드는 변경되지 않는다.

#### Scenario: 레거시 큐 보존
- **WHEN** 본 change 머지 후 `apps/api/internal/bot/priority_queue.go`를 확인할 때
- **THEN** 파일은 여전히 존재하며, 본 change에서 삭제되지 않는다(후속 정리 change에서 제거).
