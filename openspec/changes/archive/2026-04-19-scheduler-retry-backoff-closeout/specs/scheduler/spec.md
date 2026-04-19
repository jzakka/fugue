## MODIFIED Requirements

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
- **THEN** 호출 후 `fetch_error_count = 0` 이고 `last_fetched_at` 은 비-NULL 상태이며, 해당 URL이 다시 enqueue되어 실패할 경우 backoff는 첫 실패 수준에서 재시작한다.

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
- **THEN** `harvested_at` 은 호출 시각으로 갱신되어 비-NULL 상태가 되고, `harvester_frontier_pins` INSERT는 실행되지 않는다.

#### Scenario: harvest_failed status 처리
- **WHEN** Harvester consumer가 `SetStatus(..., "harvest_failed", nil)` 를 호출할 때
- **THEN** `harvester_frontier.last_updated_at`은 갱신되지만, `harvest_error_count`와 `next_harvest_at`은 SetStatus에서 변경되지 않는다.

#### Scenario: 알 수 없는 status
- **WHEN** SetStatus가 네 enum 이외의 status 문자열로 호출될 때
- **THEN** 에러가 반환되며 DB는 변경되지 않는다.

#### Scenario: 알 수 없는 key
- **WHEN** SetStatus가 frontier에 존재하지 않는 `key`로 호출될 때
- **THEN** 새 row를 만들지 않고 warn 로그만 기록한다(panic 금지).
