## ADDED Requirements

### Requirement: fetch 실패 시 exponential backoff를 next_fetch_at에 적용한다
URLScheduler는 Pioneer로부터 fetch 실패가 보고될 때 `bot_frontier` row의 `fetch_error_count`를 1 증가시키고, `next_fetch_at`을 `now() + 30s * 2^fetch_error_count_before + jitter`로 갱신해야 한다(SHALL). 여기서 `fetch_error_count_before`는 이번 실패를 반영하기 전의 값이며, `jitter`는 산출된 delay의 ±10% 범위에서 균일 분포로 표집된 값이다.

#### Scenario: 첫 fetch 실패의 backoff
- **WHEN** Pioneer가 `fetch_error_count = 0`인 row에 대한 fetch 실패를 scheduler에 보고할 때
- **THEN** 같은 row의 `fetch_error_count`가 1로 증가하고, `next_fetch_at`이 `now() + 30s ± 3s` 범위(30s의 ±10%)로 갱신된다.

#### Scenario: 두 번째 fetch 실패의 backoff
- **WHEN** `fetch_error_count = 1`인 row에 대해 추가 fetch 실패가 보고될 때
- **THEN** `fetch_error_count`가 2가 되고, `next_fetch_at`이 `now() + 60s ± 6s` 범위로 갱신된다.

#### Scenario: 네 번째 fetch 실패의 backoff
- **WHEN** `fetch_error_count = 3`인 row에 대해 추가 fetch 실패가 보고될 때
- **THEN** `fetch_error_count`가 4가 되고, `next_fetch_at`이 `now() + 240s ± 24s` 범위로 갱신된다.

#### Scenario: jitter가 균일 분포에서 표집된다
- **WHEN** 동일 `fetch_error_count_before` 값으로 다수의 실패를 보고하여 산출된 jitter 값을 관찰할 때
- **THEN** 표집된 jitter 값들이 `[-0.1*delay, +0.1*delay]` 범위에 분포하며, 두 보고가 항상 동일한 `next_fetch_at`을 갖지 않는다.

---

### Requirement: harvest 실패 시 동일한 exponential backoff를 next_harvest_at에 적용한다
URLScheduler는 Harvester로부터 harvest 실패가 보고될 때 `bot_frontier` row의 `harvest_error_count`를 1 증가시키고, `next_harvest_at`을 `now() + 30s * 2^harvest_error_count_before + jitter(±10%)`로 갱신해야 한다(SHALL). 공식은 fetch 측과 동일해야 한다(SHALL).

#### Scenario: 첫 harvest 실패의 backoff
- **WHEN** Harvester가 `harvest_error_count = 0`인 row에 대한 harvest 실패를 scheduler에 보고할 때
- **THEN** 같은 row의 `harvest_error_count`가 1로 증가하고, `next_harvest_at`이 `now() + 30s ± 3s` 범위로 갱신된다.

#### Scenario: harvest와 fetch가 동일한 base/jitter를 사용
- **WHEN** 동일한 `error_count_before` 값으로 fetch 실패와 harvest 실패의 delay 공식을 비교할 때
- **THEN** base(30s), 지수(`2^n`), jitter 비율(±10%) 모두 동일하다.

---

### Requirement: fetch 성공 시 fetch_error_count를 0으로 리셋한다
URLScheduler는 Pioneer로부터 fetch 성공이 보고될 때 해당 `bot_frontier` row의 `fetch_error_count`를 0으로 갱신해야 한다(SHALL).

#### Scenario: 누적된 실패 카운트가 성공으로 리셋된다
- **WHEN** `fetch_error_count = 3`인 row에 대해 fetch 성공이 보고될 때
- **THEN** 같은 row의 `fetch_error_count`가 0이 된다.

#### Scenario: 이미 0인 카운트는 0으로 유지
- **WHEN** `fetch_error_count = 0`인 row에 대해 fetch 성공이 보고될 때
- **THEN** `fetch_error_count`가 0으로 유지된다.

---

### Requirement: harvest 성공 시 harvest_error_count를 0으로 리셋한다
URLScheduler는 Harvester로부터 harvest 성공이 보고될 때 해당 `bot_frontier` row의 `harvest_error_count`를 0으로 갱신해야 한다(SHALL).

#### Scenario: 누적된 harvest 실패 카운트가 성공으로 리셋된다
- **WHEN** `harvest_error_count = 2`인 row에 대해 harvest 성공이 보고될 때
- **THEN** 같은 row의 `harvest_error_count`가 0이 된다.

---

### Requirement: 재시도 한도 5에 도달한 row는 dead로 취급되어 claim되지 않는다
`bot_frontier` row의 `fetch_error_count`가 5 이상이면 Pioneer claim 대상에서 영구적으로 제외되어야 한다(SHALL). `harvest_error_count`가 5 이상이면 Harvester claim 대상에서 영구적으로 제외되어야 한다(SHALL). URLScheduler는 dead 상태에 도달한 row에 대해 별도의 cleanup(삭제·아카이브)을 수행하지 않아야 한다(SHALL NOT) — cleanup은 본 capability의 책임이 아니다.

#### Scenario: 다섯 번째 fetch 실패가 row를 dead로 만든다
- **WHEN** `fetch_error_count = 4`인 row에 대해 추가 fetch 실패가 보고될 때
- **THEN** `fetch_error_count`가 5가 되고, 이후 어떤 Pioneer claim 쿼리도 해당 row를 반환하지 않는다.

#### Scenario: dead row는 next_fetch_at이 도래해도 claim되지 않는다
- **WHEN** `fetch_error_count = 5`이고 `next_fetch_at <= now()`인 row가 존재할 때
- **THEN** Pioneer가 claim을 시도해도 해당 row는 반환되지 않는다(`fetch_error_count < 5` 조건 위배).

#### Scenario: harvest 측도 동일하게 동작
- **WHEN** `harvest_error_count = 5`인 row가 존재할 때
- **THEN** Harvester claim 쿼리는 해당 row를 반환하지 않는다.

#### Scenario: dead row는 frontier에서 삭제되지 않는다
- **WHEN** row가 dead 상태에 도달한 직후 `bot_frontier` 테이블을 조회할 때
- **THEN** 해당 row가 여전히 테이블에 존재하며, scheduler는 해당 row를 자동으로 삭제하거나 다른 테이블로 이동시키지 않는다.

---

### Requirement: 에러 보고 경로가 backoff 공식을 적용한다
URLScheduler는 Pioneer/Harvester가 fetch/harvest 결과를 알리는 보고 경로(예: `SetStatus` 또는 동등한 `RecordFetchError`/`RecordHarvestError`/`RecordFetchSuccess`/`RecordHarvestSuccess` 메서드)에서 본 capability에 정의된 backoff 공식과 reset 규칙을 적용해야 한다(SHALL). Pioneer/Harvester는 위 보고 경로를 거치지 않고 `bot_frontier`의 backoff 컬럼을 직접 수정해서는 안 된다(SHALL NOT).

#### Scenario: 보고 경로 한 번의 호출이 단일 UPDATE로 반영된다
- **WHEN** 워커가 fetch 실패를 단 한 번 scheduler에 보고할 때
- **THEN** 해당 row의 `fetch_error_count`와 `next_fetch_at`이 단일 트랜잭션 안에서 일관되게 갱신된다.

#### Scenario: 워커가 직접 컬럼을 수정하지 않는다
- **WHEN** Pioneer/Harvester 코드 경로를 검토할 때
- **THEN** `fetch_error_count`, `next_fetch_at`, `harvest_error_count`, `next_harvest_at` 컬럼은 URLScheduler 보고 메서드 외부에서 직접 UPDATE되지 않는다.
