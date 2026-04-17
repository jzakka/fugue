## ADDED Requirements

### Requirement: Harvester는 URLScheduler consumer로 동작한다
Harvester는 자체적인 큐 자료구조를 보유하지 않고(SHALL NOT), `URLScheduler` 인터페이스의 consumer로만 동작해야 한다(SHALL). "다음에 처리할 노드"는 항상 `URLScheduler.Dequeue` 호출 결과로 결정되어야 한다(SHALL).

#### Scenario: Harvester 메인 루프가 Dequeue로 시작
- **WHEN** Harvester 워커가 한 iteration을 시작할 때
- **THEN** 가장 먼저 `URLScheduler.Dequeue(ctx)`를 호출하여 처리 대상 row를 획득하고, 그 외의 어떤 인메모리 큐/리스트에서도 다음 노드를 꺼내지 않는다.

#### Scenario: 자체 큐/visited/nodeMap 자료구조 부재
- **WHEN** 신규 Harvester 구현체의 필드와 함수를 정적으로 점검할 때
- **THEN** `BFSQueue`, `visited map`, 사이트 전체 노드를 사전 적재하는 `nodeMap` 등 "다음 노드 후보"를 보관하는 인메모리 자료구조가 존재하지 않는다.

#### Scenario: 그래프 순회 로직 부재
- **WHEN** Harvester가 한 row를 처리한 직후
- **THEN** 해당 row의 outgoing edge를 따라가 다음 노드를 자체 결정하지 않으며, 다음 노드 결정은 다시 `URLScheduler.Dequeue` 호출로 위임된다.

---

### Requirement: 메인 루프는 Dequeue → fetch → ParseDocument → Index → SetStatus 순서를 따른다
Harvester의 단일 iteration은 다음 단계를 순서대로 수행해야 한다(SHALL):
1. `URLScheduler.Dequeue(ctx)`로 처리 대상 URL을 claim한다.
2. 해당 URL의 HTML/콘텐츠를 fetch한다.
3. fetch된 콘텐츠를 ParseDocument(스크립트 실행 등 도메인별 추출 로직)로 콘텐츠 항목 배열로 변환한다.
4. 추출된 항목을 Index(Pin 생성을 포함한 처리 파이프라인)로 전달한다.
5. 처리 결과에 따라 frontier row 상태를 갱신한다(`URLScheduler.SetStatus` 또는 동등한 갱신 호출).

각 단계의 실패는 다음 단계 실행을 중단해야 한다(SHALL).

#### Scenario: 정상 흐름
- **WHEN** Dequeue가 URL `U`를 반환하고, fetch/ParseDocument/Index가 모두 성공할 때
- **THEN** Harvester는 위 순서대로 5단계를 모두 수행하고, 마지막에 frontier row의 성공 상태를 기록한 뒤 다음 iteration을 시작한다.

#### Scenario: fetch 실패 시 후속 단계 스킵
- **WHEN** fetch 단계가 에러를 반환할 때
- **THEN** ParseDocument와 Index는 호출되지 않으며, frontier row는 실패 상태로만 갱신된다.

#### Scenario: ParseDocument 실패 시 Index 스킵
- **WHEN** fetch는 성공했으나 ParseDocument가 에러를 반환하거나 항목 0개를 반환할 때
- **THEN** Index는 호출되지 않으며 (또는 빈 입력으로 호출되어 무동작), frontier row는 실패 상태로 갱신된다.

#### Scenario: Index 실패 시 frontier 실패 처리
- **WHEN** fetch와 ParseDocument는 성공했으나 Index 단계에서 모든 항목이 처리 실패할 때
- **THEN** frontier row는 실패 상태로 갱신되고, `pin_id`는 NULL로 유지된다.

---

### Requirement: 인메모리 진행 상태를 보유하지 않는다
Harvester 프로세스는 어떤 진행 상태(이미 처리한 노드 ID 집합, 사이트별 진행률, 다음 처리 예정 노드 후보 등)도 인메모리에만 보관해서는 안 된다(SHALL NOT). 모든 공유 상태는 `bot_frontier` 또는 다른 영속 저장소에 보관되어야 한다(SHALL).

#### Scenario: 워커 재시작 시 진행 상태 보존
- **WHEN** Harvester 워커 프로세스가 SIGTERM/크래시로 중단되었다가 재시작될 때
- **THEN** 이전에 성공 처리된 row는 `pin_id`가 채워진 상태로 frontier에 남아 다시 claim되지 않으며, 처리 중이던 row는 트랜잭션 롤백으로 다시 claim 가능 상태로 복원된다.

#### Scenario: 사이트별 visited/nodeMap 부재
- **WHEN** 워커가 동작 중인 임의 시점에 메모리 사용량을 점검할 때
- **THEN** 사이트별 노드 사전 적재(`nodeMap`)나 사이트별 `visited` 집합 등 사이트 단위로 비례하여 증가하는 자료구조가 존재하지 않는다.

#### Scenario: 다음 처리 후보를 메모리에 누적하지 않음
- **WHEN** 워커가 한 iteration 처리를 완료한 직후
- **THEN** 다음 iteration 후보 URL은 메모리에 보관되지 않으며, 항상 다음 `Dequeue` 호출로 새로 획득한다.

---

### Requirement: 다중 워커 정확성은 URLScheduler에 위임한다
Harvester는 동일 row가 두 워커에 동시 dequeue되는 것을 방지하는 락/큐 정확성 로직을 자체 구현하지 않아야 한다(SHALL NOT). 해당 정확성은 `URLScheduler`(예: `FOR UPDATE SKIP LOCKED` 기반 claim)가 보장한다고 가정해야 한다(SHALL).

#### Scenario: Harvester 자체 락 부재
- **WHEN** Harvester 구현체를 점검할 때
- **THEN** Harvester가 직접 잡는 advisory lock, 분산 락, 워커 간 조정 채널이 존재하지 않으며, 정확성 보장은 `URLScheduler.Dequeue`의 계약에 의존한다.

#### Scenario: 임의 워커 수에서 동시 실행 안전
- **WHEN** Harvester 워커 N개(N >= 2)가 동시에 실행될 때
- **THEN** 동일 `normalized_url`이 두 워커에 동시 dequeue되지 않고, 동일 row에 대해 최대 한 번만 Pin 생성 시도가 일어난다 (정확성 자체는 scheduler 계약이 보장).

#### Scenario: 사이트 경계 무관 동시 처리
- **WHEN** 한 워커가 사이트 A의 row를 처리하는 동안 다른 워커가 사이트 B의 row를 동시에 처리할 때
- **THEN** 두 처리는 서로 간섭하지 않으며, Harvester는 사이트 단위 동기화를 시도하지 않는다.

---

### Requirement: 성공 처리 시 frontier row의 pin_id를 갱신한다
Harvester가 fetch → ParseDocument → Index를 거쳐 Pin을 생성한 경우, 해당 row의 `pin_id` 컬럼을 생성된 Pin ID로 갱신해야 한다(SHALL). 갱신 후 해당 row는 Harvester partial index(`pin_id IS NULL AND ...`)에서 자동 제외되어 다시 claim되지 않아야 한다(SHALL).

#### Scenario: Pin 생성 직후 pin_id 갱신
- **WHEN** Index 단계에서 Pin이 1건 이상 정상 생성될 때
- **THEN** Harvester는 `URLScheduler.SetStatus`(또는 동등한 갱신)로 해당 row의 `pin_id`를 채우고, 동일 row가 후속 `Dequeue`로 반환되지 않는다.

#### Scenario: 전부 중복 스킵된 경우의 pin_id 처리
- **WHEN** 추출된 모든 항목이 봇 중복 체크로 스킵되어 신규 Pin이 0건 생성될 때
- **THEN** Harvester는 해당 row를 "처리 완료(다시 claim 불필요)" 상태로 표시한다 — 구체적으로는 frontier row의 `pin_id` 또는 동등한 종료 마커를 갱신하여 partial index에서 제외시킨다.

#### Scenario: 다중 항목 추출 시 대표 pin_id 선정
- **WHEN** 한 row 처리에서 여러 Pin이 생성될 때
- **THEN** 그 중 하나의 Pin ID를 frontier row의 `pin_id`로 기록한다 (어느 Pin을 대표로 할지의 정확한 규칙은 본 spec에서 강제하지 않으며, "비-NULL이고 유효한 Pin ID"이면 충분하다).

---

### Requirement: 실패 처리 시 harvest_error_count를 증가시킨다
Harvester가 한 row 처리 중 fetch/ParseDocument/Index 어느 단계에서든 실패할 경우, 해당 row의 `harvest_error_count`를 1 증가시키고 `next_harvest_at`을 백오프 시각으로 갱신해야 한다(SHALL). `pin_id`는 NULL로 유지해야 한다(SHALL).

#### Scenario: fetch 실패 시 카운터 증가
- **WHEN** fetch 단계가 에러(타임아웃, 4xx/5xx, 네트워크 오류 등)를 반환할 때
- **THEN** 해당 row의 `harvest_error_count`가 1 증가하고, `next_harvest_at`은 미래 시각으로 갱신되며, `pin_id`는 NULL로 유지된다.

#### Scenario: ParseDocument 실패 시 카운터 증가
- **WHEN** 스크립트 실행이 구문/런타임/타임아웃 에러로 실패할 때
- **THEN** 해당 row의 `harvest_error_count`가 1 증가한다.

#### Scenario: Index 전체 실패 시 카운터 증가
- **WHEN** 추출된 모든 항목의 Pin 생성이 DB 에러 등으로 실패할 때
- **THEN** 해당 row의 `harvest_error_count`가 1 증가한다.

#### Scenario: 한도 도달 시 partial index에서 제외
- **WHEN** `harvest_error_count`가 5에 도달할 때
- **THEN** 해당 row는 Harvester partial index에서 제외되어 더 이상 `Dequeue`로 반환되지 않는다 (이 동작 자체는 `scheduler-frontier-table`이 보장).

---

### Requirement: Harvester는 사이트 경계와 무관하게 동작한다
Harvester는 한 번의 실행에서 단일 사이트만 처리한다는 가정을 가져서는 안 된다(SHALL NOT). 한 워커가 여러 사이트의 row를 임의로 섞어 처리할 수 있어야 하며(SHALL), 사이트별 상태(루트 노드, 사이트별 진행률 등)를 메인 루프에서 유지해서는 안 된다(SHALL NOT).

#### Scenario: 단일 워커가 여러 사이트 row 처리
- **WHEN** frontier에 사이트 A와 사이트 B의 처리 대기 row가 모두 존재하고 워커가 연속해서 `Dequeue`를 호출할 때
- **THEN** 우선순위(`score DESC`)에 따라 A와 B의 row가 임의 순서로 반환되며, Harvester는 사이트가 바뀐다는 사실에 대해 어떤 특별 처리도 하지 않는다.

#### Scenario: 사이트 루트 노드 탐색 부재
- **WHEN** Harvester 메인 루프 코드를 점검할 때
- **THEN** 처리 시작 시 사이트 루트 노드를 찾는 단계(`findRootNode` 등)가 존재하지 않는다. 처리 단위는 `Dequeue`가 반환한 단일 row 그 자체다.

---

### Requirement: Dequeue가 비어 있을 때의 동작
`URLScheduler.Dequeue`가 즉시 처리 가능한 row가 없음을 알리는 신호(빈 결과/에러/블록 해제)를 반환할 경우, Harvester는 짧은 대기 후 재호출하는 polling 동작을 수행하거나 scheduler가 제공하는 blocking 동작을 그대로 사용해야 한다(SHALL). 이때 Harvester는 자체 큐를 채우거나 그래프를 따라 임의 노드를 처리하지 않아야 한다(SHALL NOT).

#### Scenario: 빈 큐에서의 polling
- **WHEN** `Dequeue`가 빈 결과를 반환할 때
- **THEN** Harvester는 짧은 백오프 후 다시 `Dequeue`를 호출하며, 그 사이에 자체 노드 후보를 만들거나 이전 노드의 edge를 따라가지 않는다.

#### Scenario: 컨텍스트 취소 시 종료
- **WHEN** `Dequeue` 대기 중 ctx가 취소될 때
- **THEN** Harvester는 현재 iteration을 안전하게 종료하고 워커 루프를 빠져나간다.
