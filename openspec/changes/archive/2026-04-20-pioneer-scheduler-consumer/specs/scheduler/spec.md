## MODIFIED Requirements

### Requirement: 신규 enqueue된 row는 즉시 처리 가능한 초기 상태를 가진다
Pioneer가 새 URL을 `pioneer_frontier`에 enqueue할 때, 또는 Pioneer가 fetch에 성공한 URL을 `harvester_frontier`에 fanout할 때, 신규 row는 즉시 처리 가능한 초기 상태로 생성되어야 한다(SHALL).

본 change는 기존 scenario 중 두 개를 개정한다. (1) `pioneer_frontier`의 신규 INSERT는 항상 `depth = 0`으로 기록되며, 부모 row 기반 `depth + 1` 전파는 더 이상 수행되지 않는다(Pioneer가 부모 관계를 추적하지 않는 새 consumer 모델과 일관). 구조화된 enqueue 경로에서의 depth 전파는 후속 change의 범위로 남는다. (2) `harvester_frontier`의 UPSERT 시 `snapshot_key`를 세팅하는 경로는 `EnqueueHarvester(url, snapshotKey)`이며, baseline `Enqueue(QueueHarvester, urls...)` 경로는 `snapshot_key`를 건드리지 않는다(baseline 규약 유지).

#### Scenario: pioneer_frontier 초기 상태
- **WHEN** Pioneer가 새 URL을 `pioneer_frontier`에 INSERT할 때
- **THEN** `last_fetched_at IS NULL`, `fetch_error_count = 0`, `next_fetch_at <= now()` 상태로 생성되어 partial index에 포함된다.

#### Scenario: pioneer_frontier depth 초기값
- **WHEN** Pioneer가 발견한 링크를 `pioneer_frontier`에 enqueue할 때
- **THEN** 신규 INSERT row의 `depth`는 항상 `0`으로 기록된다 (Pioneer는 부모-자식 관계를 추적하지 않으며, BFS depth 전파는 후속 change의 구조화된 enqueue 경로에서 다룬다).

#### Scenario: harvester_frontier 초기 상태
- **WHEN** Pioneer가 fetch에 성공하여 `EnqueueHarvester(url, snapshotKey)`로 `harvester_frontier`에 UPSERT할 때
- **THEN** (신규 INSERT일 때) `harvested_at IS NULL`, `harvest_error_count = 0`, `snapshot_key`는 호출 인자 `snapshotKey`로 세팅, `next_harvest_at <= now()` 상태로 생성되어 partial index에 포함된다 (baseline `Enqueue(QueueHarvester, urls...)` 경로의 `snapshot_key` 미변경 규약과의 분리는 본 change ADDED Requirement의 "baseline Enqueue와의 분리" scenario가 별도 담당).

---

## ADDED Requirements

### Requirement: URLScheduler는 EnqueueHarvester(url, snapshotKey) 메서드를 제공한다
`URLScheduler` 인터페이스는 다음 메서드를 추가로 제공해야 한다(SHALL):

- `EnqueueHarvester(url string, snapshotKey string) error` — `url`을 `harvester_frontier`에 UPSERT하고, 동일 호출에서 `snapshot_key` 컬럼을 `snapshotKey`로 세팅한다.

본 메서드는 baseline의 `Enqueue(QueueHarvester, urls...)`와 병행 제공되며, 서로 다른 두 가지 호출 상황을 분리한다.
- `Enqueue(QueueHarvester, urls...)`는 URL만 전달하는 기존 경로로, `snapshot_key`를 건드리지 않는다(baseline 규약 유지). 본 change 적용 후 프로덕션 코드 경로에 이 메서드의 호출자는 없으며(Pioneer는 항상 `EnqueueHarvester`를 사용), 향후 재시도 재투입·운영 도구 등 보조 enqueue 용도로 보존된다.
- `EnqueueHarvester(url, snapshotKey)`는 Pioneer consumer가 fetch 직후 snapshot을 저장한 뒤 snapshot_key까지 함께 기록해야 하는 상황을 위한 경로다.

UPSERT 동작:
- **이미 `harvested_at IS NOT NULL`인 row**에 대해서는 no-op으로 동작해야 한다(SHALL). 재harvest를 유발해서는 안 된다(SHALL NOT).
- **`harvested_at IS NULL`인 row**(신규 또는 미완료)에 대해서는 `snapshot_key`를 `snapshotKey`로 갱신하고, `next_harvest_at`을 재enqueue 시점으로 갱신하며, `harvest_error_count`를 0으로 초기화해야 한다(SHALL).
- Postgres unique constraint violation을 호출자에게 노출해서는 안 된다(SHALL NOT).

#### Scenario: 미존재 URL에 대한 EnqueueHarvester는 새 row를 생성한다
- **WHEN** `harvester_frontier`에 해당 `url_hash` row가 없는 상태에서 `EnqueueHarvester(url, snapshotKey)`가 호출될 때
- **THEN** 새 row가 생성되고 `snapshot_key`는 호출 인자 값으로 세팅되며, `next_harvest_at`은 호출 시각으로 설정된다

#### Scenario: 이미 harvest된 URL은 no-op이다
- **WHEN** 동일 `url_hash`의 row가 이미 `harvested_at IS NOT NULL`인 상태에서 `EnqueueHarvester(url, snapshotKey)`가 호출될 때
- **THEN** `snapshot_key` / `next_harvest_at` / `harvest_error_count` 어느 컬럼도 변경되지 않는다 (재harvest 방지 가드)

#### Scenario: 미완료 URL에 대한 EnqueueHarvester는 snapshot_key를 갱신한다
- **WHEN** 동일 `url_hash`의 row가 존재하고 `harvested_at IS NULL`인 상태에서 `EnqueueHarvester(url, snapshotKey)`가 호출될 때
- **THEN** 해당 row의 `snapshot_key`는 호출 인자 값으로 갱신되고, `next_harvest_at`이 호출 시각으로 갱신되며, `harvest_error_count`는 0으로 리셋된다

#### Scenario: unique violation 미노출
- **WHEN** 호출자가 `EnqueueHarvester`를 사용할 때
- **THEN** Postgres unique constraint violation 에러는 호출자에게 노출되지 않는다

#### Scenario: baseline Enqueue와의 분리
- **WHEN** 호출자가 `Enqueue(QueueHarvester, url)`를 호출했을 때
- **THEN** 해당 경로는 `snapshot_key`를 변경하지 않는다 (baseline의 "Enqueue는 snapshot_key를 건드리지 않는다" 규약 유지). snapshot_key 기록이 필요한 호출자는 `EnqueueHarvester`를 사용해야 한다
