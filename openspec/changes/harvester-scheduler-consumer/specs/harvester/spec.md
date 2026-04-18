## ADDED Requirements

### Requirement: Harvester는 Dequeue(QueueHarvester)로만 URL을 수급한다
Harvester 워커는 자체적인 큐/그래프 순회 구조를 보유하지 않아야 하며(SHALL NOT), 처리할 URL은 오직 `scheduler.Dequeue(scheduler.QueueHarvester)` 호출 결과로만 획득해야 한다(SHALL). 단일 `Dequeue` 호출은 `harvester_frontier`의 partial index(`WHERE harvested_at IS NULL AND harvest_error_count < 5`)를 대상으로 한 claim이며, 다른 테이블이나 인메모리 캐시를 참조하지 않는다.

#### Scenario: 메인 루프는 Dequeue(QueueHarvester)로 시작
- **WHEN** Harvester 워커가 한 iteration을 시작할 때
- **THEN** 가장 먼저 `scheduler.Dequeue(scheduler.QueueHarvester)`를 호출하여 처리 대상 URL을 획득하고, 그 외의 어떤 인메모리 큐/리스트에서도 다음 URL을 꺼내지 않는다.

#### Scenario: 다른 큐 타입을 사용하지 않음
- **WHEN** Harvester 구현체를 점검할 때
- **THEN** `Dequeue(QueuePioneer)` 호출이나 `pioneer_frontier` 직접 조회가 존재하지 않는다.

#### Scenario: 자체 큐/visited/nodeMap 자료구조 부재
- **WHEN** 신규 Harvester 구현체의 필드와 함수를 정적으로 점검할 때
- **THEN** `BFSQueue`, `visited map`, 사이트 전체 노드를 사전 적재하는 `nodeMap` 등 "다음 URL 후보"를 보관하는 인메모리 자료구조가 존재하지 않는다.

#### Scenario: 그래프 순회 로직 부재
- **WHEN** Harvester가 한 URL을 처리한 직후
- **THEN** 해당 URL의 outgoing edge를 따라가 다음 URL을 자체 결정하지 않으며, 다음 URL 결정은 다시 `scheduler.Dequeue(scheduler.QueueHarvester)` 호출로 위임된다.

---

### Requirement: 메인 루프는 snapshot-first fetch → PinDocument → Pin 생성 → SetStatus 순서를 따른다
Harvester의 단일 iteration은 다음 단계를 순서대로 수행해야 한다(SHALL):
1. `scheduler.Dequeue(scheduler.QueueHarvester)`로 처리 대상 URL을 claim한다.
2. `harvester-snapshot-first-fetch` capability가 제공하는 snapshot-first 경로로 HTML을 획득한다(snapshot_key가 있으면 snapshot 우선, miss 시 HTTP live fetch).
3. `harvester-pin-document` capability의 `harvestPipeline.Process`로 HTML을 `PinDocument`로 파싱한다.
4. `PinDocument.Pinnable`이 true이면 Pin을 생성하여 `pinIDs []int64`를 수집한다. false이면 Pin 생성을 건너뛴다.
5. 성공 시 `scheduler.SetStatus(url, "harvested", pinIDs)`를 호출한다. `pinIDs`가 nil 또는 빈 슬라이스이면 매핑 없이 완료 표기한다.

각 단계의 실패는 다음 단계 실행을 중단해야 하며(SHALL), 실패 처리(본 spec의 별도 requirement)를 따라야 한다.

#### Scenario: 정상 흐름 - Pin 1건 생성
- **WHEN** `Dequeue`가 URL `U`를 반환하고, snapshot-first fetch와 PinDocument 파싱이 성공하고 `Pinnable = true`이며 Pin 1건이 생성될 때
- **THEN** Harvester는 `scheduler.SetStatus(U, "harvested", []int64{pinID})`를 호출한 뒤 다음 iteration을 시작한다.

#### Scenario: 정상 흐름 - Pin N건 생성
- **WHEN** `PinDocument`가 복수 Pin으로 materialize되어 `pinIDs`가 길이 N(N>=2)인 슬라이스일 때
- **THEN** Harvester는 `scheduler.SetStatus(U, "harvested", pinIDs)`를 **단일 호출**로 전달하고, scheduler 구현이 `harvested_at` UPDATE와 `harvester_frontier_pins` 일괄 INSERT를 한 트랜잭션에서 처리한다.

#### Scenario: Pinnable = false 시 Pin 생성 스킵
- **WHEN** fetch와 파싱은 성공했으나 `PinDocument.Pinnable == false`일 때
- **THEN** Harvester는 Pin을 생성하지 않고 `scheduler.SetStatus(U, "harvested", nil)`을 호출하여 해당 row를 완료 상태로 표기한다. `harvester_frontier_pins`에는 아무 row도 INSERT되지 않는다.

#### Scenario: 빈 pinIDs 슬라이스도 완료 표기로 처리
- **WHEN** `pinIDs`가 nil이거나 길이 0인 슬라이스일 때
- **THEN** `SetStatus(U, "harvested", nil)` 호출과 동일하게 처리되어 매핑 없이 `harvested_at`만 갱신된다.

---

### Requirement: 성공 상태 전이는 harvested_at UPDATE와 harvester_frontier_pins INSERT를 한 트랜잭션으로 처리한다
Harvester consumer가 호출하는 `scheduler.SetStatus(url, "harvested", pinIDs)`는 다음 두 작업을 단일 DB 트랜잭션에서 수행해야 한다(SHALL):
- `harvester_frontier` row의 `harvested_at`을 `now()`로 UPDATE.
- `pinIDs`의 각 원소에 대해 `harvester_frontier_pins(frontier_id, pin_id)`에 INSERT.

두 작업이 분리되어 "매핑 없는 harvested row" 또는 "harvested_at이 NULL인데 매핑이 있는 row"가 생겨서는 안 된다(SHALL NOT).

#### Scenario: 성공 트랜잭션 원자성
- **WHEN** `SetStatus(U, "harvested", []int64{p1, p2, p3})` 호출 중 어느 한 INSERT가 실패할 때
- **THEN** `harvested_at` UPDATE를 포함한 전체 트랜잭션이 롤백되어, 다음 `Dequeue`에서 동일 row가 다시 반환될 수 있다.

#### Scenario: Pin 0건 성공의 매핑 부재
- **WHEN** `SetStatus(U, "harvested", nil)`이 호출될 때
- **THEN** `harvested_at`은 갱신되고 `harvester_frontier_pins`에는 아무 row도 INSERT되지 않으며, partial index에서 해당 row가 제외된다.

#### Scenario: harvested row는 partial index에서 제외
- **WHEN** `SetStatus(U, "harvested", ...)` 호출이 성공한 직후
- **THEN** 동일 URL은 다음 `Dequeue(QueueHarvester)` 호출로 반환되지 않는다 (partial index의 `WHERE harvested_at IS NULL` 조건).

---

### Requirement: 실패 시 SetStatus + RecordHarvestError를 둘 다 호출한다
Harvester가 fetch/파싱/Pin 생성 중 어느 단계에서 실패하든, 해당 URL에 대해 다음 두 호출을 순서대로 수행해야 한다(SHALL):
1. `scheduler.SetStatus(url, "harvest_failed", nil)` — 상태 전이 표기.
2. `scheduler.RecordHarvestError(url, errorKind)` — 카운터 누적과 backoff 적용.

`errorKind`는 다음 중 하나여야 한다(SHALL): `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"`, `"parse"`, `"pin_create"`.

#### Scenario: HTTP 4xx 응답 시 errorKind = http_4xx
- **WHEN** snapshot miss 후 live fetch가 HTTP 4xx를 반환할 때
- **THEN** `SetStatus(U, "harvest_failed", nil)`과 `RecordHarvestError(U, "http_4xx")`가 이 순서로 호출된다.

#### Scenario: HTTP 5xx 응답 시 errorKind = http_5xx
- **WHEN** fetch가 HTTP 5xx를 반환할 때
- **THEN** `RecordHarvestError(U, "http_5xx")`가 호출된다.

#### Scenario: DNS/connect/TLS 실패 시 errorKind = network
- **WHEN** fetch가 DNS 해석/TCP connect/TLS handshake 실패로 종료될 때
- **THEN** `RecordHarvestError(U, "network")`가 호출된다.

#### Scenario: 타임아웃 시 errorKind = timeout
- **WHEN** fetch 또는 스크립트 실행이 타임아웃으로 종료될 때
- **THEN** `RecordHarvestError(U, "timeout")`가 호출된다.

#### Scenario: 파싱 실패 시 errorKind = parse
- **WHEN** `harvestPipeline.Process`가 스크립트 구문/런타임 에러 등으로 실패할 때
- **THEN** `RecordHarvestError(U, "parse")`가 호출된다.

#### Scenario: Pin 생성 실패 시 errorKind = pin_create
- **WHEN** DB 에러 등으로 Pin INSERT가 실패할 때
- **THEN** `RecordHarvestError(U, "pin_create")`가 호출된다.

#### Scenario: SetStatus와 RecordHarvestError 둘 다 호출 보장
- **WHEN** 어떤 실패 경로로든 iteration이 종료될 때
- **THEN** `SetStatus("harvest_failed", nil)`과 `RecordHarvestError`가 모두 호출된 상태로 다음 iteration으로 넘어간다. 둘 중 하나만 호출하고 종료하지 않는다.

---

### Requirement: 재harvest를 수행하지 않는다 (UPSERT guard에 의존)
Harvester는 "이미 harvest된 URL인지" 직접 확인하는 코드를 포함하지 않아야 한다(SHALL NOT). 재harvest 방지는 `scheduler-frontier-table`이 정의한 UPSERT guard(`WHERE harvester_frontier.harvested_at IS NULL`)에 의해 스키마 수준에서 달성된다고 가정해야 한다(SHALL).

#### Scenario: Pioneer 재크롤 후에도 재harvest 안 함
- **WHEN** Pioneer가 동일 URL을 재크롤하여 `harvester_frontier`에 UPSERT를 시도했을 때 (해당 row는 이미 `harvested_at IS NOT NULL`)
- **THEN** UPSERT guard로 `next_harvest_at`/`snapshot_key`가 덮어써지지 않으며, partial index의 `WHERE harvested_at IS NULL` 조건에 의해 `Dequeue`에 반환되지 않는다.

#### Scenario: Harvester 코드 내부에 harvested 체크 부재
- **WHEN** Harvester 구현체를 정적으로 점검할 때
- **THEN** "이미 harvested인지"를 SELECT하는 경로가 존재하지 않는다. 중복 방지는 partial index와 UPSERT guard로 일원화된다.

---

### Requirement: 다중 워커 정확성은 URLScheduler에 위임한다
Harvester는 동일 URL이 두 워커에 동시 dequeue되는 것을 방지하는 락/큐 정확성 로직을 자체 구현하지 않아야 한다(SHALL NOT). 해당 정확성은 `scheduler-claim-api`의 `FOR UPDATE SKIP LOCKED` 기반 claim이 보장한다고 가정해야 한다(SHALL).

#### Scenario: Harvester 자체 락 부재
- **WHEN** Harvester 구현체를 점검할 때
- **THEN** Harvester가 직접 잡는 advisory lock, 분산 락, 워커 간 조정 채널이 존재하지 않으며, 정확성 보장은 `scheduler.Dequeue`의 계약에 의존한다.

#### Scenario: 임의 워커 수에서 동시 실행 안전
- **WHEN** Harvester 워커 N개(N >= 2)가 동시에 실행될 때
- **THEN** 동일 `normalized_url`이 두 워커에 동시 dequeue되지 않고, 동일 row에 대해 최대 한 번만 Pin 생성 시도가 일어난다 (정확성 자체는 scheduler 계약이 보장).

---

### Requirement: Harvester는 사이트 경계와 무관하게 동작한다
Harvester는 `queue_type`이 harvester인 모든 URL을 host와 무관하게 처리해야 한다(SHALL). 한 워커가 한 번의 실행에서 단일 사이트만 처리한다는 가정을 가져서는 안 된다(SHALL NOT). 사이트별 상태(루트 노드, 사이트별 진행률 등)를 메인 루프에서 유지해서는 안 된다(SHALL NOT).

#### Scenario: 단일 워커가 여러 host row 처리
- **WHEN** `harvester_frontier`에 host A와 host B의 처리 대기 row가 모두 존재하고 워커가 연속해서 `Dequeue`를 호출할 때
- **THEN** 우선순위(`score DESC, next_harvest_at ASC`)에 따라 A와 B의 row가 임의 순서로 반환되며, Harvester는 host가 바뀐다는 사실에 대해 어떤 특별 처리도 하지 않는다.

#### Scenario: 사이트 루트 노드 탐색 부재
- **WHEN** Harvester 메인 루프 코드를 점검할 때
- **THEN** 처리 시작 시 사이트 루트 노드를 찾는 단계(`findRootNode` 등)가 존재하지 않는다. 처리 단위는 `Dequeue`가 반환한 단일 URL 그 자체다.

---

### Requirement: 빈 큐 polling은 Dequeue 내부 책임이다
Harvester consumer 루프는 빈 큐 처리를 위한 자체 sleep/backoff 로직을 가져서는 안 된다(SHALL NOT). `scheduler.Dequeue(QueueHarvester)`는 내부에서 polling(빈 결과 시 1초 sleep 후 재시도)을 수행하고, URL이 claim되기 전에는 return하지 않는 blocking 시그니처여야 한다(SHALL, `scheduler-claim-api`가 보장). 예외는 `ctx` 취소뿐이다.

#### Scenario: consumer 루프에 sleep 호출 부재
- **WHEN** Harvester consumer 루프 코드를 정적으로 점검할 때
- **THEN** `time.Sleep`, `time.After` 등의 자체 polling backoff 호출이 존재하지 않는다. 빈 큐 재시도는 `Dequeue` 내부에서만 발생한다.

#### Scenario: 컨텍스트 취소 시 안전 종료
- **WHEN** `Dequeue` 대기 중 `ctx`가 취소될 때
- **THEN** `Dequeue`가 에러를 반환하고, Harvester는 현재 iteration을 안전하게 종료하여 워커 루프를 빠져나간다.

---

### Requirement: 인메모리 진행 상태를 보유하지 않는다
Harvester 프로세스는 어떤 진행 상태(이미 처리한 URL 집합, 사이트별 진행률, 다음 처리 예정 URL 후보 등)도 인메모리에만 보관해서는 안 된다(SHALL NOT). 모든 공유 상태는 `harvester_frontier`/`harvester_frontier_pins` 또는 다른 영속 저장소에 보관되어야 한다(SHALL).

#### Scenario: 워커 재시작 시 진행 상태 보존
- **WHEN** Harvester 워커 프로세스가 SIGTERM/크래시로 중단되었다가 재시작될 때
- **THEN** 이전에 성공 처리된 row는 `harvested_at`이 채워진 상태로 frontier에 남아 다시 claim되지 않으며, 처리 중이던 row는 트랜잭션 롤백 + lease timeout으로 다시 claim 가능 상태로 복원된다.

#### Scenario: 사이트별 visited/nodeMap 부재
- **WHEN** 워커가 동작 중인 임의 시점에 메모리 사용량을 점검할 때
- **THEN** 사이트별 노드 사전 적재(`nodeMap`)나 사이트별 `visited` 집합 등 사이트 단위로 비례하여 증가하는 자료구조가 존재하지 않는다.

#### Scenario: 다음 처리 후보를 메모리에 누적하지 않음
- **WHEN** 워커가 한 iteration 처리를 완료한 직후
- **THEN** 다음 iteration 후보 URL은 메모리에 보관되지 않으며, 항상 다음 `Dequeue` 호출로 새로 획득한다.
