## ADDED Requirements

### Requirement: Frontier 테이블이 Pioneer/Harvester 공유 상태를 단일 row로 보관한다
시스템은 크롤링 대상 URL의 fetch 상태와 harvest 상태를 단일 Postgres 테이블 `bot_frontier`의 한 row로 보관해야 한다(SHALL). 별도의 `status` enum 컬럼은 두지 않아야 하며(SHALL NOT), 상태는 `last_fetched_at`, `pin_id`, `fetch_error_count`, `harvest_error_count`, `next_fetch_at`, `next_harvest_at` 컬럼의 조합으로 표현해야 한다(SHALL).

#### Scenario: 신규 enqueue된 URL의 초기 상태
- **WHEN** Pioneer가 새 URL을 frontier에 enqueue할 때
- **THEN** `bot_frontier` row가 생성되고, `last_fetched_at IS NULL`, `pin_id IS NULL`, `fetch_error_count = 0`, `harvest_error_count = 0`이며, `next_fetch_at`과 `next_harvest_at`은 즉시 처리 가능한 시각(예: `now()` 또는 그 이전)으로 설정된다.

#### Scenario: fetch 완료 후의 상태
- **WHEN** Pioneer가 URL의 HTML을 성공적으로 가져온 직후 frontier row를 갱신할 때
- **THEN** 동일 row의 `last_fetched_at`이 fetch 시각으로 채워지고 `pin_id`는 여전히 NULL이다.

#### Scenario: harvest 완료 후의 상태
- **WHEN** Harvester가 row의 콘텐츠로 Pin을 생성한 직후 frontier row를 갱신할 때
- **THEN** 동일 row의 `pin_id`가 생성된 Pin ID로 채워진다.

#### Scenario: status 컬럼 부재
- **WHEN** `bot_frontier` 테이블 스키마를 확인할 때
- **THEN** `status` 라는 이름의 enum/text 컬럼이 존재하지 않는다.

---

### Requirement: normalized_url에 대한 unique 제약으로 중복 enqueue를 방지한다
`bot_frontier` 테이블은 `normalized_url` 컬럼에 unique constraint를 가져야 한다(SHALL). 동일 `normalized_url`을 두 번 INSERT하면 DB 레벨에서 거부되어야 한다(SHALL).

#### Scenario: 동일 URL 중복 enqueue 거부
- **WHEN** 어떤 워커가 이미 존재하는 `normalized_url`로 INSERT를 시도할 때
- **THEN** unique constraint violation 에러가 반환되고, 새 row는 생성되지 않으며, 기존 row는 변경되지 않는다.

#### Scenario: 정규화 차이가 사라진 두 URL
- **WHEN** Pioneer가 `https://example.com/page?utm_source=x`와 `https://example.com/page`를 모두 enqueue 시도하고 두 URL의 `normalized_url`이 동일할 때
- **THEN** 두 번째 INSERT는 unique constraint로 거부되어 단 하나의 row만 존재한다.

---

### Requirement: 컬럼 정의가 Pioneer/Harvester 양쪽 동작을 포괄한다
`bot_frontier` 테이블은 다음 컬럼을 가져야 한다(SHALL):
- `normalized_url` (정규화된 URL, unique)
- `url` (원본 URL)
- `host` (URL의 호스트, 보조 인덱스 대상)
- `depth` (사이트 진입점으로부터의 BFS depth)
- `score` (우선순위 점수)
- `last_fetched_at` (Pioneer가 마지막으로 fetch한 시각, NULL 가능)
- `next_fetch_at` (다음 fetch 가능 시각, 백오프용)
- `fetch_error_count` (fetch 실패 누적 횟수)
- `pin_id` (Harvester가 생성한 Pin ID, NULL 가능)
- `next_harvest_at` (다음 harvest 가능 시각, 백오프용)
- `harvest_error_count` (harvest 실패 누적 횟수)
- `last_updated_at` (row 마지막 갱신 시각)

#### Scenario: 모든 컬럼 존재 확인
- **WHEN** `bot_frontier` 테이블의 컬럼 목록을 조회할 때
- **THEN** 위에 나열된 모든 컬럼명이 존재한다.

#### Scenario: nullable 정책
- **WHEN** 컬럼의 nullable 여부를 확인할 때
- **THEN** `last_fetched_at`과 `pin_id`는 NULL을 허용하고, 그 외 위 목록의 컬럼은 NOT NULL이다.

#### Scenario: pin_id는 Harvester가 채운다
- **WHEN** 신규 row가 생성된 직후
- **THEN** `pin_id`는 NULL이며, Harvester가 Pin 생성을 완료한 뒤에만 비-NULL 값으로 갱신된다.

---

### Requirement: Pioneer claim용 partial index가 존재한다
`bot_frontier` 테이블은 Pioneer의 claim 쿼리를 인덱스만으로 처리할 수 있는 partial index를 가져야 한다(SHALL). partial WHERE 절은 `last_fetched_at IS NULL AND fetch_error_count < 5`을 포함해야 하며(SHALL), 정렬 키는 `score DESC`를 우선으로 해야 한다(SHALL).

#### Scenario: 인덱스 존재 확인
- **WHEN** `bot_frontier` 테이블의 인덱스 목록을 조회할 때
- **THEN** WHERE 절에 `last_fetched_at IS NULL`과 `fetch_error_count < 5`를 포함하고 `score DESC` 정렬을 가진 partial index가 존재한다.

#### Scenario: claim 쿼리가 인덱스를 사용
- **WHEN** Pioneer가 `WHERE last_fetched_at IS NULL AND fetch_error_count < 5 AND next_fetch_at <= now() ORDER BY score DESC LIMIT N` 형태의 쿼리를 실행할 때
- **THEN** EXPLAIN 결과가 해당 partial index를 스캔한다.

#### Scenario: fetch가 끝난 row는 인덱스에서 제외
- **WHEN** Pioneer가 row의 `last_fetched_at`을 비-NULL로 갱신할 때
- **THEN** 해당 row는 Pioneer claim용 partial index에서 제거된다.

#### Scenario: 재시도 한도를 넘은 row는 인덱스에서 제외
- **WHEN** row의 `fetch_error_count`가 5에 도달할 때
- **THEN** 해당 row는 Pioneer claim용 partial index에서 제거된다.

---

### Requirement: Harvester claim용 partial index가 존재한다
`bot_frontier` 테이블은 Harvester의 claim 쿼리를 인덱스만으로 처리할 수 있는 partial index를 가져야 한다(SHALL). partial WHERE 절은 `pin_id IS NULL AND harvest_error_count < 5`을 포함해야 하며(SHALL), 정렬 키는 `score DESC`를 우선으로 해야 한다(SHALL).

#### Scenario: 인덱스 존재 확인
- **WHEN** `bot_frontier` 테이블의 인덱스 목록을 조회할 때
- **THEN** WHERE 절에 `pin_id IS NULL`과 `harvest_error_count < 5`를 포함하고 `score DESC` 정렬을 가진 partial index가 존재한다.

#### Scenario: claim 쿼리가 인덱스를 사용
- **WHEN** Harvester가 `WHERE pin_id IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now() ORDER BY score DESC LIMIT N` 형태의 쿼리를 실행할 때
- **THEN** EXPLAIN 결과가 해당 partial index를 스캔한다.

#### Scenario: Pin 생성이 끝난 row는 인덱스에서 제외
- **WHEN** Harvester가 row의 `pin_id`를 비-NULL로 갱신할 때
- **THEN** 해당 row는 Harvester claim용 partial index에서 제거된다.

#### Scenario: 재시도 한도를 넘은 row는 인덱스에서 제외
- **WHEN** row의 `harvest_error_count`가 5에 도달할 때
- **THEN** 해당 row는 Harvester claim용 partial index에서 제거된다.

---

### Requirement: host 및 score 보조 인덱스가 존재한다
`bot_frontier` 테이블은 host별 조회와 우선순위 정렬을 지원하기 위해 `host` 컬럼과 `score` 컬럼에 각각 보조 인덱스를 가져야 한다(SHALL).

#### Scenario: host 인덱스 존재
- **WHEN** `bot_frontier` 테이블의 인덱스 목록을 조회할 때
- **THEN** `host` 컬럼을 키로 하는 인덱스가 존재한다.

#### Scenario: score 인덱스 존재
- **WHEN** `bot_frontier` 테이블의 인덱스 목록을 조회할 때
- **THEN** `score` 컬럼을 키로 하는 인덱스가 존재한다 (Pioneer/Harvester partial index와는 별도).

#### Scenario: host별 카운트 쿼리 지원
- **WHEN** 운영자가 `SELECT host, count(*) FROM bot_frontier WHERE last_fetched_at IS NULL GROUP BY host` 형태의 쿼리를 실행할 때
- **THEN** EXPLAIN 결과가 `host` 인덱스를 활용한다.

---

### Requirement: 인메모리 큐 상태를 frontier에 두지 않는다
Pioneer/Harvester는 복수 프로세스로 실행될 수 있으므로, BFS 진행 상태(visited, queue, depth 진척도 등)를 인메모리 자료구조에만 보관해서는 안 된다(SHALL NOT). 프로세스 간 공유가 필요한 모든 상태는 `bot_frontier`(또는 다른 영속 저장소)에 보관되어야 한다(SHALL).

#### Scenario: 프로세스 재시작 후 진행 보존
- **WHEN** Pioneer 프로세스가 중단되었다가 재시작될 때
- **THEN** 이전에 enqueue된 URL과 fetch 진행 상태가 `bot_frontier`에서 복원되며, 루트부터 다시 탐색하지 않는다.

#### Scenario: 두 Pioneer 인스턴스가 동시 실행
- **WHEN** Pioneer 두 인스턴스가 동시에 동일한 사이트에서 frontier를 claim할 때
- **THEN** 동일 `normalized_url`이 두 인스턴스에 동시에 dequeue되지 않는다 (구체적인 claim 쿼리/락 전략은 후속 change `scheduler-claim-api`에서 정의).
