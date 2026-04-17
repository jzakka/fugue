## Requirements

### Requirement: Pioneer 큐와 Harvester 큐를 독립된 두 테이블로 보관한다
시스템은 Pioneer의 fetch 대기 URL과 Harvester의 harvest 대기 URL을 서로 다른 Postgres 테이블 `pioneer_frontier`와 `harvester_frontier`에 각각 보관해야 한다(SHALL). 두 테이블은 `queue_type` 같은 구분 컬럼으로 합쳐지지 않아야 한다(SHALL NOT). 두 테이블 중 어느 쪽에도 `status` enum/text 컬럼은 존재하지 않아야 한다(SHALL NOT).

#### Scenario: 두 테이블이 각각 존재한다
- **WHEN** 운영자가 DB 스키마를 조회할 때
- **THEN** `pioneer_frontier`와 `harvester_frontier` 두 테이블이 모두 존재하고, 각자의 컬럼 집합이 서로 다르다.

#### Scenario: status 컬럼 부재
- **WHEN** 두 frontier 테이블의 컬럼 목록을 조회할 때
- **THEN** `status`라는 이름의 enum/text 컬럼이 존재하지 않으며, 상태는 시간/카운터 컬럼 조합으로 표현된다.

---

### Requirement: `pioneer_frontier`는 Pioneer fetch 큐의 컬럼/제약을 갖는다
`pioneer_frontier` 테이블은 다음 컬럼을 가져야 한다(SHALL): `id`, `normalized_url`, `url`, `url_hash`, `host`, `depth`, `score`, `last_fetched_at`, `next_fetch_at`, `fetch_error_count`, `last_updated_at`. `url_hash`는 `sha256(normalized_url)`의 32바이트 BYTEA이며(SHALL), `UNIQUE(url_hash)` 제약을 가져야 한다(SHALL). `score`는 `DOUBLE PRECISION`이며 0.0~1.0 범위의 값을 가진다(SHALL). `last_fetched_at`만 NULL 허용이며 나머지는 NOT NULL이다(SHALL).

#### Scenario: 컬럼 및 타입
- **WHEN** `pioneer_frontier` 스키마를 확인할 때
- **THEN** 위 모든 컬럼이 존재하고, `url_hash`는 BYTEA, `score`는 DOUBLE PRECISION, 시각 컬럼은 TIMESTAMPTZ이다.

#### Scenario: url_hash unique
- **WHEN** 동일 `url_hash`로 두 번 INSERT를 시도할 때
- **THEN** unique constraint violation이 발생하고 두 번째 INSERT는 거부된다.

#### Scenario: url_hash 길이
- **WHEN** `url_hash`에 32바이트가 아닌 값을 INSERT하려 할 때
- **THEN** DB의 길이 CHECK 제약에 의해 거부되며, 애플리케이션은 항상 `sha256(normalized_url)`의 32바이트 결과만 기록한다.

#### Scenario: 호스트 컬럼 형식
- **WHEN** URL의 host를 `host` 컬럼에 기록할 때
- **THEN** 호스트명만 저장되며(포트 제외), 대소문자는 원본을 유지하고, `www.` prefix도 원본 그대로 유지된다.

---

### Requirement: `pioneer_frontier`는 Pioneer claim용 partial index를 가진다
`pioneer_frontier`는 `fetch_error_count < 5` 조건의 partial index를 가져야 하며(SHALL), 정렬 키는 `score DESC, next_fetch_at ASC`이다(SHALL).

#### Scenario: 인덱스 정의
- **WHEN** `pioneer_frontier`의 인덱스 목록을 조회할 때
- **THEN** `WHERE fetch_error_count < 5 ORDER BY score DESC, next_fetch_at ASC` 형태의 partial index가 존재한다.

#### Scenario: claim 쿼리가 인덱스를 사용
- **WHEN** Pioneer claim 쿼리(`WHERE fetch_error_count < 5 AND next_fetch_at <= now() ORDER BY score DESC, next_fetch_at ASC LIMIT N`)를 EXPLAIN할 때
- **THEN** 위 partial index를 스캔한다.

#### Scenario: 재시도 한도 초과 row는 인덱스에서 제외
- **WHEN** row의 `fetch_error_count`가 5에 도달할 때
- **THEN** 해당 row는 Pioneer claim용 partial index에서 제외되어 다시 claim되지 않는다.

---

### Requirement: `harvester_frontier`는 Harvester harvest 큐의 컬럼/제약을 갖는다
`harvester_frontier` 테이블은 다음 컬럼을 가져야 한다(SHALL): `id`, `normalized_url`, `url`, `url_hash`, `host`, `snapshot_key`, `score`, `harvested_at`, `next_harvest_at`, `harvest_error_count`, `last_updated_at`. `url_hash`는 `sha256(normalized_url)`의 32바이트 BYTEA이며(SHALL), `UNIQUE(url_hash)` 제약을 가져야 한다(SHALL). `snapshot_key`와 `harvested_at`만 NULL 허용이며 나머지는 NOT NULL이다(SHALL). `harvester_frontier`는 `depth` 컬럼을 두지 않아야 한다(SHALL NOT).

#### Scenario: 컬럼 및 타입
- **WHEN** `harvester_frontier` 스키마를 확인할 때
- **THEN** 위 모든 컬럼이 존재하고 `depth` 컬럼은 존재하지 않는다.

#### Scenario: url_hash unique
- **WHEN** 동일 `url_hash`로 두 번 INSERT를 시도할 때
- **THEN** unique constraint violation이 발생한다.

#### Scenario: url_hash 길이
- **WHEN** `url_hash`에 32바이트가 아닌 값을 INSERT하려 할 때
- **THEN** DB의 길이 CHECK 제약에 의해 거부되며, 애플리케이션은 항상 `sha256(normalized_url)`의 32바이트 결과만 기록한다.

#### Scenario: 호스트 컬럼 형식
- **WHEN** URL의 host를 `host` 컬럼에 기록할 때
- **THEN** 호스트명만 저장되며(포트 제외), 대소문자는 원본을 유지하고, `www.` prefix도 원본 그대로 유지된다. `pioneer_frontier`와 동일 규칙이 적용된다.

---

### Requirement: `harvester_frontier`는 Harvester claim용 partial index를 가진다
`harvester_frontier`는 `harvested_at IS NULL AND harvest_error_count < 5` 조건의 partial index를 가져야 하며(SHALL), 정렬 키는 `score DESC, next_harvest_at ASC`이다(SHALL).

#### Scenario: 인덱스 정의
- **WHEN** `harvester_frontier`의 인덱스 목록을 조회할 때
- **THEN** `WHERE harvested_at IS NULL AND harvest_error_count < 5 ORDER BY score DESC, next_harvest_at ASC` 형태의 partial index가 존재한다.

#### Scenario: claim 쿼리가 인덱스를 사용
- **WHEN** Harvester claim 쿼리(`WHERE harvested_at IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now() ORDER BY score DESC, next_harvest_at ASC LIMIT N`)를 EXPLAIN할 때
- **THEN** 위 partial index를 스캔한다.

#### Scenario: harvest 완료 row는 인덱스에서 제외
- **WHEN** row의 `harvested_at`이 비-NULL로 갱신될 때
- **THEN** 해당 row는 Harvester claim용 partial index에서 제외된다.

---

### Requirement: `harvester_frontier_pins` 조인 테이블이 1:N 관계를 표현한다
시스템은 `harvester_frontier` 한 row가 생성한 여러 Pin과의 관계를 별도의 조인 테이블 `harvester_frontier_pins`에 기록해야 한다(SHALL). 두 외래 키 (`frontier_id`, `pin_id`)가 복합 primary key이며(SHALL), 양쪽 모두 `ON DELETE CASCADE`로 선언되어야 한다(SHALL). `pin_id`는 기존 `pins.id`와 동일한 `UUID` 타입을 가지며(SHALL), `frontier_id`는 `harvester_frontier.id`의 `BIGINT` 타입을 가진다(SHALL).

#### Scenario: 테이블 정의
- **WHEN** 스키마를 조회할 때
- **THEN** `harvester_frontier_pins(frontier_id BIGINT, pin_id UUID)` 테이블이 존재하고, `frontier_id`는 `harvester_frontier(id)`를, `pin_id`는 `pins(id)`를 참조하며, 복합 PK를 가진다.

#### Scenario: frontier row 삭제 시 조인 row 자동 삭제
- **WHEN** `harvester_frontier`의 row가 삭제될 때
- **THEN** 해당 row를 참조하던 `harvester_frontier_pins` row가 CASCADE로 함께 삭제된다.

#### Scenario: pin 삭제 시 조인 row 자동 삭제
- **WHEN** `pins`의 row가 삭제될 때
- **THEN** 해당 row를 참조하던 `harvester_frontier_pins` row가 CASCADE로 함께 삭제된다.

#### Scenario: 한 frontier row가 여러 Pin과 연결
- **WHEN** Harvester가 하나의 `harvester_frontier` row를 처리하여 Pin 여러 개를 생성할 때
- **THEN** 동일 `frontier_id`를 공유하는 여러 `harvester_frontier_pins` row가 생성된다.

---

### Requirement: 신규 enqueue된 row는 즉시 처리 가능한 초기 상태를 가진다
Pioneer가 새 URL을 `pioneer_frontier`에 enqueue할 때, 또는 Pioneer가 fetch에 성공한 URL을 `harvester_frontier`에 fanout할 때, 신규 row는 즉시 처리 가능한 초기 상태로 생성되어야 한다(SHALL).

#### Scenario: pioneer_frontier 초기 상태
- **WHEN** Pioneer가 새 URL을 `pioneer_frontier`에 INSERT할 때
- **THEN** `last_fetched_at IS NULL`, `fetch_error_count = 0`, `next_fetch_at <= now()` 상태로 생성되어 partial index에 포함된다.

#### Scenario: pioneer_frontier depth 초기값
- **WHEN** Pioneer가 발견한 링크를 `pioneer_frontier`에 enqueue할 때
- **THEN** 해당 URL이 루트로 등록되는 경우 `depth = 0`으로, 기존 row에서 파생된 링크인 경우 부모 row의 `depth + 1`로 기록되어 BFS 순서가 보존된다.

#### Scenario: harvester_frontier 초기 상태
- **WHEN** Pioneer가 fetch에 성공하여 `harvester_frontier`에 UPSERT할 때
- **THEN** (신규 INSERT일 때) `harvested_at IS NULL`, `harvest_error_count = 0`, `snapshot_key`는 Pioneer가 저장한 키로 세팅, `next_harvest_at <= now()` 상태로 생성되어 partial index에 포함된다.

---

### Requirement: `next_fetch_at` / `next_harvest_at`은 in-flight lease marker를 겸한다
시스템은 row를 claim할 때 별도의 `claimed_at`/`lease_until` 컬럼을 두지 않고, `next_fetch_at` (또는 `next_harvest_at`)을 `now() + 10 minutes`로 UPDATE하여 lease marker로 사용해야 한다(SHALL). lease timeout은 10분이다(SHALL). lease 기간 동안 같은 row는 다른 워커가 다시 claim하지 못해야 한다(SHALL NOT).

#### Scenario: claim 시 lease marker 갱신
- **WHEN** 워커가 `pioneer_frontier` row를 claim할 때
- **THEN** 해당 row의 `next_fetch_at`이 `now() + 10 minutes`로 UPDATE되어, 다른 워커의 claim 쿼리(`next_fetch_at <= now()`)에서 제외된다.

#### Scenario: harvester claim 시 lease marker 갱신
- **WHEN** 워커가 `harvester_frontier` row를 claim할 때
- **THEN** 해당 row의 `next_harvest_at`이 `now() + 10 minutes`로 UPDATE된다.

#### Scenario: 워커 크래시 후 자동 회수
- **WHEN** 어떤 워커가 row를 claim한 후 10분이 지나도록 결과를 기록하지 못하고 죽을 때
- **THEN** lease marker가 만료되어 해당 row의 `next_*_at <= now()`가 다시 성립하고, 다른 워커가 회수한다.

---

### Requirement: Pioneer fetch 성공 시 365일 뒤 재크롤로 스케줄한다
Pioneer가 URL fetch에 성공하고 결과를 기록할 때, 시스템은 `pioneer_frontier` row의 `last_fetched_at = now()`, `fetch_error_count = 0`, `next_fetch_at = now() + 365 days`로 UPDATE해야 한다(SHALL).

#### Scenario: fetch 성공 후 상태
- **WHEN** Pioneer가 URL의 HTML을 성공적으로 가져와 결과를 기록할 때
- **THEN** 해당 row의 `last_fetched_at`은 현재 시각으로 채워지고, `next_fetch_at`은 1년 뒤로, `fetch_error_count`는 0으로 세팅된다. 해당 row는 1년 동안 Pioneer claim 대상에서 제외된다.

---

### Requirement: Pioneer fetch 실패 시 에러 카운터와 backoff를 갱신한다
Pioneer가 URL fetch에 실패할 때, 시스템은 `fetch_error_count`를 1 증가시키고 `next_fetch_at`을 backoff 공식에 따라 미래로 갱신해야 한다(SHALL). 카운터가 5에 도달하면 해당 row는 partial index에서 제외되어 더 이상 claim되지 않는다(SHALL NOT). 구체적인 backoff 공식은 후속 change `scheduler-retry-backoff`에서 정의한다.

#### Scenario: 일시적 실패
- **WHEN** Pioneer가 fetch에 실패하여 결과를 기록할 때
- **THEN** `fetch_error_count`가 1 증가하고, `next_fetch_at`이 미래 시각으로 UPDATE된다.

#### Scenario: 재시도 한도 도달
- **WHEN** `fetch_error_count`가 5에 도달할 때
- **THEN** 해당 row는 `pioneer_frontier` partial index에서 제외되어 다시 claim되지 않는다.

---

### Requirement: Harvester는 harvested_at이 NULL인 row에 대해서만 UPSERT로 동작한다
Pioneer가 재크롤에 성공하여 `harvester_frontier`에 UPSERT를 시도할 때, 시스템은 `ON CONFLICT (url_hash) DO UPDATE ... WHERE harvester_frontier.harvested_at IS NULL` 조건을 적용하여 이미 harvest된 URL은 no-op 처리해야 한다(SHALL). 재harvest는 수행하지 않는다(SHALL NOT).

#### Scenario: 이미 harvest된 URL 재fetch
- **WHEN** Pioneer가 이미 `harvested_at IS NOT NULL`인 URL을 다시 fetch하여 `harvester_frontier`에 UPSERT를 시도할 때
- **THEN** ON CONFLICT DO UPDATE가 `harvested_at IS NULL` 조건으로 걸려 있어 해당 row는 갱신되지 않고, 재harvest도 트리거되지 않는다.

#### Scenario: 아직 harvest되지 않은 URL 재fetch
- **WHEN** Pioneer가 `harvested_at IS NULL` 상태의 기존 URL을 다시 fetch하여 UPSERT를 시도할 때
- **THEN** `snapshot_key`, `next_harvest_at`, `harvest_error_count` 등이 UPDATE되어 Harvester가 새 snapshot으로 재시도할 수 있다.

---

### Requirement: Harvester 성공 시 harvested_at을 세팅하고 생성된 Pin을 조인 테이블에 기록한다
Harvester가 row 처리에 성공할 때, 시스템은 동일 트랜잭션 안에서 `harvester_frontier.harvested_at = now()`로 UPDATE하고, 생성된 모든 Pin ID를 `harvester_frontier_pins (frontier_id, pin_id)`에 INSERT해야 한다(SHALL).

#### Scenario: Pin 1개 생성
- **WHEN** Harvester가 row를 처리하여 Pin 1개를 생성할 때
- **THEN** `harvester_frontier.harvested_at`이 현재 시각으로 UPDATE되고, `harvester_frontier_pins`에 (frontier_id, pin_id) row 1개가 INSERT된다.

#### Scenario: Pin 여러 개 생성
- **WHEN** Harvester가 row를 처리하여 Pin N개를 생성할 때
- **THEN** 동일 `frontier_id`와 각각 다른 `pin_id`를 가진 `harvester_frontier_pins` row가 N개 INSERT된다.

#### Scenario: 이후 동일 row 재-claim 방지
- **WHEN** `harvested_at`이 세팅된 후
- **THEN** 해당 row는 Harvester claim용 partial index에서 제외되어 다시 claim되지 않는다.

---

### Requirement: Harvester 실패 시 에러 카운터와 backoff를 갱신한다
Harvester가 row 처리에 실패할 때, 시스템은 `harvest_error_count`를 1 증가시키고 `next_harvest_at`을 backoff 공식에 따라 미래로 갱신해야 한다(SHALL). 카운터가 5에 도달하면 해당 row는 partial index에서 제외된다(SHALL NOT). 구체적인 backoff 공식은 후속 change `scheduler-retry-backoff`에서 정의한다.

#### Scenario: 일시적 실패
- **WHEN** Harvester가 처리에 실패하여 결과를 기록할 때
- **THEN** `harvest_error_count`가 1 증가하고, `next_harvest_at`이 미래 시각으로 UPDATE되며, `harvested_at`은 여전히 NULL이다.

#### Scenario: 재시도 한도 도달
- **WHEN** `harvest_error_count`가 5에 도달할 때
- **THEN** 해당 row는 `harvester_frontier` partial index에서 제외되어 다시 claim되지 않는다.

---

### Requirement: 인메모리 큐 상태를 frontier에 두지 않는다
Pioneer/Harvester는 복수 프로세스로 실행될 수 있으므로, BFS 진행 상태(visited, queue, depth 진척도 등)를 인메모리 자료구조에만 보관해서는 안 된다(SHALL NOT). 프로세스 간 공유가 필요한 모든 상태는 `pioneer_frontier` / `harvester_frontier` (또는 다른 영속 저장소)에 보관되어야 한다(SHALL).

#### Scenario: 프로세스 재시작 후 진행 보존
- **WHEN** Pioneer 프로세스가 중단되었다가 재시작될 때
- **THEN** 이전에 enqueue된 URL과 fetch 진행 상태가 `pioneer_frontier`에서 복원되며, 루트부터 다시 탐색하지 않는다.

#### Scenario: 두 Pioneer 인스턴스가 동시 실행
- **WHEN** Pioneer 두 인스턴스가 동시에 동일한 사이트에서 row를 claim할 때
- **THEN** `next_fetch_at` lease marker 규약에 따라 동일 row가 두 인스턴스에 동시에 넘어가지 않는다 (구체적인 claim 쿼리/락 전략은 후속 change `scheduler-claim-api`에서 정의).
