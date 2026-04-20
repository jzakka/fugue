## Purpose

Fugue의 크롤링 파이프라인(Pioneer→Harvester)을 위한 URL 스케줄러 capability. Postgres 기반 frontier 테이블(`pioneer_frontier`, `harvester_frontier`)의 스키마/제약/lease 규약과, 호스트별 politeness(token bucket)를 포함한 dequeue 정책을 정의한다. 인메모리 큐를 영속 frontier로 대체하여 수평 확장 가능한 다중 워커 운영을 지원한다.
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

### Requirement: 스케줄러는 호스트별 token bucket으로 politeness를 적용한다
스케줄러는 호스트별 token bucket을 유지하고, 외부 소비자가 특정 호스트의 token 가용성을 질의할 수 있는 허용-여부 질의 동작을 제공해야 한다(SHALL). 처음 보는 호스트에 대한 허용-여부 질의는 기본 rate/burst로 bucket을 lazy 생성한 뒤 판정해야 한다(SHALL). Token bucket은 프로세스별 인메모리 자료구조로 유지되어야 하며(SHALL), 프로세스 간 token 상태를 공유하지 않아야 한다(SHALL NOT).

호스트 키의 형식은 `scheduler-frontier-table`의 `host` 컬럼 값과 동일하다: 호스트명만(포트 제외), 대소문자 원본 유지, `www.` prefix 유지. 정규화 책임은 호출부(Pioneer)에 있으며, scheduler는 전달된 문자열을 그대로 키로 사용한다(SHALL).

claim 시점의 허용-여부 질의 호출 타이밍(예: `SELECT FOR UPDATE SKIP LOCKED` 후 각 후보 row의 host에 대한 검사)은 `scheduler-claim-api` capability의 Claim 프로토콜에서 정의되며, 본 capability는 동작의 행위 계약만 제공한다.

#### Scenario: token이 있는 호스트에 대한 허용-여부 질의는 true를 반환한다
- **WHEN** 호출자가 호스트 `H`에 대해 허용-여부 질의를 호출하고 호스트 `H`의 bucket에 token이 1개 이상 남아 있을 때
- **THEN** 허용-여부 질의는 `true`를 반환하고 호스트 `H`의 bucket에서 token 1개가 소비된다.

#### Scenario: token이 없는 호스트에 대한 허용-여부 질의는 false를 반환한다
- **WHEN** 호스트 `H`의 bucket이 token 0개 상태에서 허용-여부 질의가 호출될 때
- **THEN** 허용-여부 질의는 `false`를 반환하고 token은 소비되지 않는다.

#### Scenario: 프로세스 재시작 후 token 상태는 초기화된다
- **WHEN** 스케줄러 프로세스가 중단되었다가 재시작될 때
- **THEN** 모든 호스트의 token bucket은 burst 한도까지 가득 찬 초기 상태에서 시작한다(인메모리 상태이므로 보존되지 않는다).

#### Scenario: 두 스케줄러 프로세스의 token 상태는 독립이다
- **WHEN** 동일 호스트 `H`에 대해 두 개의 스케줄러 프로세스가 동시에 실행될 때
- **THEN** 각 프로세스가 독립된 token bucket을 가지며, 한 프로세스의 token 소비가 다른 프로세스의 가용 token에 영향을 주지 않는다.

#### Scenario: 호스트 키는 frontier host 컬럼 값 그대로 사용된다
- **WHEN** 호출자가 `www.Example.COM` 호스트에 대해 허용-여부 질의를 호출하고 scheduler 내부에 동일 문자열 키로 bucket이 존재할 때
- **THEN** 해당 bucket이 조회되며 scheduler는 대소문자/`www.` prefix를 변형하지 않는다.

---

### Requirement: 기본 rate와 burst는 설정 가능하다
스케줄러는 호스트별 기본 token 충전율(rate)과 최대 누적 token 수(burst)를 설정값으로 노출해야 한다(SHALL). 운영자가 별도 설정하지 않았을 때의 공장 기본값은 호스트당 1 req/sec rate와 burst 5여야 한다(SHALL). 처음 조회되는 호스트의 bucket은 운영자 설정 기본값(미설정 시 공장 기본값)으로 lazy 생성되어야 한다(SHALL).

#### Scenario: 공장 기본값으로 새 호스트 bucket이 생성된다
- **WHEN** 운영자가 별도 설정을 하지 않은 상태에서 처음 본 호스트 `H`에 대해 허용-여부 질의가 호출될 때
- **THEN** 호스트 `H`의 bucket이 rate=1 req/sec, burst=5로 생성되고 첫 허용-여부 질의는 true를 반환한다.

#### Scenario: 운영자가 기본 rate/burst를 변경한다
- **WHEN** 운영자가 설정값 `scheduler.host_default_rate_per_sec`을 0.5로, `scheduler.host_default_burst`를 3으로 변경하고 스케줄러를 재기동할 때
- **THEN** 그 이후 처음 등장하는 호스트의 bucket은 rate=0.5 req/sec, burst=3으로 생성된다.

---

### Requirement: 호스트별 rate/burst를 외부에서 override할 수 있다
스케줄러는 특정 호스트의 rate와 burst를 런타임에 변경할 수 있는 호스트 rate/burst 설정 동작을 제공해야 한다(SHALL). 호출 후 즉시 해당 호스트의 bucket이 새 설정으로 동작해야 한다(SHALL). 기존 호스트의 경우 새 rate/burst로 교체하고, 처음 보는 호스트의 경우 지정된 설정으로 신규 bucket을 생성해야 한다(SHALL).

#### Scenario: 호스트 override가 즉시 반영된다
- **WHEN** 호출자가 호스트 `example.com`에 rate=0.1, burst=1로 호스트 rate/burst 설정 동작을 호출한 직후 허용-여부 질의가 호출될 때
- **THEN** 호스트 `example.com`의 bucket은 rate=0.1 req/sec, burst=1로 동작하며, 연속 허용-여부 질의 시 burst 1 이후로는 false를 반환한다.

#### Scenario: 처음 보는 호스트에 대한 override는 신규 bucket을 생성한다
- **WHEN** 처음 보는 호스트 `new.example.org`에 대해 rate=2.0, burst=10으로 호스트 rate/burst 설정 동작이 호출될 때
- **THEN** 해당 호스트의 bucket이 rate=2.0 req/sec, burst=10으로 생성되며 운영자 기본값/공장 기본값을 사용하지 않는다.

---

### Requirement: rate/burst 유효성 검사와 안전한 기본값 대체
스케줄러의 호스트 rate/burst 설정 동작은 `rate <= 0` 또는 `burst <= 0`인 입력을 받으면 해당 호스트 bucket을 **현재 운영자 설정 기본값(`scheduler.host_default_rate_per_sec`, `scheduler.host_default_burst`)**으로 대체하고 `WARN` 레벨 경고 로그를 남겨야 한다(SHALL). 운영자가 별도 설정을 하지 않은 경우 공장 기본값(1 req/sec, burst 5)이 적용된다(SHALL). 이 경우 에러를 반환하거나 패닉을 발생시키지 않아야 하며(SHALL NOT), 서비스는 중단 없이 계속 동작해야 한다(SHALL). 유효성 대체 후의 허용-여부 질의 동작은 대체된 기본값 bucket 기준으로 수행되어야 한다(SHALL).

#### Scenario: rate가 0이면 운영자 기본값으로 대체된다
- **WHEN** 운영자가 별도 설정을 하지 않은 상태에서 호출자가 호스트 `bad.example.com`에 rate=0, burst=5로 호스트 rate/burst 설정 동작을 호출할 때
- **THEN** `bad.example.com`의 bucket이 공장 기본값(rate=1 req/sec, burst=5)으로 생성되고 `WARN` 로그가 남으며, 호출은 에러 없이 반환된다.

#### Scenario: burst가 음수이면 운영자가 변경한 기본값으로 대체된다
- **WHEN** 운영자가 `scheduler.host_default_rate_per_sec=0.5`, `scheduler.host_default_burst=3`으로 변경한 환경에서 호출자가 호스트 `bad.example.com`에 rate=0.5, burst=-1로 호스트 rate/burst 설정 동작을 호출할 때
- **THEN** `bad.example.com`의 bucket이 운영자 기본값(rate=0.5 req/sec, burst=3)으로 생성되고 `WARN` 로그가 남는다.

#### Scenario: 유효하지 않은 입력이어도 서비스는 중단되지 않는다
- **WHEN** 호스트 rate/burst 설정 동작에 rate=-1, burst=0을 전달한 후 곧바로 `bad.example.com` 호스트에 대해 허용-여부 질의가 호출될 때
- **THEN** 허용-여부 질의는 운영자 기본값(미설정 시 공장 기본값) bucket을 기준으로 정상 판정해 true 또는 false를 반환한다(패닉/에러 없음).

---

### Requirement: robots.txt Crawl-delay를 호스트 rate로 반영한다
스케줄러는 robots.txt에서 추출된 Crawl-delay 값을 호스트의 token bucket rate로 반영할 수 있는 진입점으로 호스트 rate/burst 설정 동작을 제공해야 한다(SHALL). 본 capability는 robots.txt 자체의 fetch/파싱을 수행하지 않는다(SHALL NOT). robots.txt 파싱과 호스트 rate/burst 설정 동작 호출 타이밍은 `pioneer-link-filter-policy` capability의 책임이다.

#### Scenario: Crawl-delay가 rate로 환산되어 적용된다
- **WHEN** Pioneer가 robots.txt에서 호스트 `slow.example.com`의 Crawl-delay = 10초를 파싱하고 스케줄러에 rate=0.1, burst=1(=1/10 req/sec)로 호스트 rate/burst 설정 동작을 호출할 때
- **THEN** 스케줄러는 호스트 `slow.example.com`의 token bucket을 rate=0.1 req/sec, burst=1로 운영한다.

#### Scenario: 호스트 rate/burst 설정 동작이 호출되지 않은 호스트는 기본값으로 동작한다
- **WHEN** scheduler가 호스트 `H`에 대해 호스트 rate/burst 설정 동작을 한 번도 호출받지 않은 상태에서 `H`에 대한 허용-여부 질의가 호출될 때
- **THEN** 호스트 `H`의 bucket은 운영자 설정 기본값(미설정 시 공장 기본값인 rate=1 req/sec, burst=5)으로 lazy 생성되어 동작한다.

---

### Requirement: token bucket 검사를 비활성화할 수 있다
스케줄러는 호스트별 token bucket 검사를 전역적으로 비활성화할 수 있는 운영 설정을 제공해야 한다(SHALL). 비활성화된 상태에서는 모든 호스트에 대한 허용-여부 질의가 항상 `true`를 반환해야 하며(SHALL), 비활성화된 상태 동안 어떤 호스트의 bucket 내부 상태(token 잔량, 생성 시각)도 변경하지 않아야 한다(SHALL NOT). 이 설정은 운영 중 문제 발생 시 즉시 롤백 수단으로 사용된다.

#### Scenario: 비활성화 시 허용-여부 질의는 항상 true를 반환한다
- **WHEN** token bucket 검사 비활성화 설정이 켜진 상태에서 임의 호스트 `H`에 대해 허용-여부 질의가 호출될 때
- **THEN** 호스트 `H`의 bucket 상태(빈/가득/없음)와 무관하게 허용-여부 질의는 `true`를 반환한다.

#### Scenario: 비활성화 상태에서 토큰 소비가 발생하지 않는다
- **WHEN** 활성화 상태에서 호스트 `H`의 bucket이 burst 한도까지 가득 찬 후, 비활성화 상태로 전환되어 동일 호스트 `H`에 대해 연속 100회 허용-여부 질의가 호출되고, 다시 활성화 상태로 전환된 직후 호스트 `H`에 대해 허용-여부 질의가 호출될 때
- **THEN** 비활성 기간 100회 모두 `true`를 반환하며, 활성화 직후 호스트 `H`의 bucket은 여전히 burst 한도까지 가득 찬 상태이다(비활성 동안 token이 소비되지 않음).

---

### Requirement: URLScheduler는 실패 보고 API RecordFetchError / RecordHarvestError를 제공한다
URLScheduler는 실패 경로를 위한 두 개의 실패 보고 API를 제공해야 한다(SHALL):

- Pioneer가 `key`(= `normalized_url`)에 대한 fetch 실패를 보고하는 `RecordFetchError`.
- Harvester가 동일 형식으로 harvest 실패를 보고하는 `RecordHarvestError`.

두 API의 구체 시그니처(Go 타입, context 파라미터 등)는 `scheduler-claim-api`의 `URLScheduler` interface 정의에 따른다. 본 capability는 실패 보고 시 발생해야 하는 관찰 가능한 행위만 정의한다.

`errorKind`는 다음 enum 중 하나여야 한다(SHALL): `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"`. 열거 외 값은 에러를 반환해야 하며(SHALL), row를 변경해서는 안 된다(SHALL NOT).

성공 경로의 `fetch_error_count` / `harvest_error_count` reset은 본 API가 아니라 `scheduler-claim-api`의 `SetStatus`가 담당한다(참조: `scheduler-claim-api` spec의 `SetStatus` 요구사항). 본 capability는 실패 경로만 정의한다.

#### Scenario: fetch 실패 보고
- **WHEN** Pioneer가 `"https://example.com/x"` 키와 `"http_5xx"` errorKind로 `RecordFetchError`를 호출할 때
- **THEN** scheduler가 해당 row에 본 capability가 정의한 backoff 공식을 적용한다.

#### Scenario: harvest 실패 보고
- **WHEN** Harvester가 `"https://example.com/x"` 키와 `"timeout"` errorKind로 `RecordHarvestError`를 호출할 때
- **THEN** scheduler가 해당 row에 동일한 backoff 공식을 적용한다.

#### Scenario: 알 수 없는 errorKind 거부 (fetch)
- **WHEN** `RecordFetchError`가 `"unknown"` errorKind로 호출될 때
- **THEN** API가 에러를 반환하고, 해당 row의 `fetch_error_count` / `next_fetch_at`은 변경되지 않는다.

#### Scenario: 알 수 없는 errorKind 거부 (harvest)
- **WHEN** `RecordHarvestError`가 `"unknown"` errorKind로 호출될 때
- **THEN** API가 에러를 반환하고, 해당 row의 `harvest_error_count` / `next_harvest_at`은 변경되지 않는다.

---

### Requirement: http_4xx 에러는 즉시 dead 처리된다
`errorKind == "http_4xx"`로 `RecordFetchError`가 호출되면 scheduler는 해당 row의 `fetch_error_count`를 **공식 적용 없이 즉시 5로 설정**해야 한다(SHALL). backoff 공식(`30s * 2^n`)은 이 경로에서 적용되지 않아야 한다(SHALL NOT). Harvester 측(`RecordHarvestError`, `harvest_error_count`)도 동일하다(SHALL).

이유: 4xx(404/410/401/403 등)는 재시도해도 회복 가능성이 없는 결정적 실패이므로 5회 재시도 비용을 소비하지 않는다.

4xx 경로에서 `next_fetch_at` / `next_harvest_at`은 갱신되지 않아야 한다(SHALL NOT) — 해당 row는 dead로 전환되어 partial index에서 제외되므로 backoff 타임스탬프는 의미가 없으며, `next_fetch_at` / `next_harvest_at`은 기존 값을 그대로 유지한다. (본 "기존 값 유지" 규칙은 `next_*_at` 컬럼에만 한정된다. `last_updated_at`은 본 capability의 별도 요구사항에 따라 갱신된다.)

#### Scenario: 4xx 첫 호출에 즉시 dead
- **WHEN** `fetch_error_count = 0`인 row에 대해 `"http_4xx"` errorKind로 `RecordFetchError`가 호출될 때
- **THEN** 같은 row의 `fetch_error_count`가 5로 설정되고, 이후 Pioneer claim 쿼리는 해당 row를 반환하지 않는다.

#### Scenario: 4xx는 backoff 공식을 건너뛴다
- **WHEN** `fetch_error_count = 2`인 row에 대해 `"http_4xx"` errorKind로 `RecordFetchError`가 호출될 때
- **THEN** `fetch_error_count`가 3이 아니라 5로 설정된다(공식의 `+=1` 증가가 아님).

#### Scenario: 4xx는 next_fetch_at을 변경하지 않는다
- **WHEN** 호출 직전 `next_fetch_at = T0`인 row에 대해 `"http_4xx"` errorKind로 `RecordFetchError`가 호출될 때 (`T0`는 보통 `Dequeue`가 설정한 lease marker 값 `T_claim + 10분`이지만 어떤 값이든 무방)
- **THEN** 호출 직후 `next_fetch_at`은 여전히 `T0`이다(dead로 claim되지 않으므로 무의미하지만 기존 값 유지).

#### Scenario: harvest 측 4xx도 동일
- **WHEN** `harvest_error_count = 1`인 row에 대해 `"http_4xx"` errorKind로 `RecordHarvestError`가 호출될 때
- **THEN** `harvest_error_count`가 5로 설정되고, `next_harvest_at`은 기존 값을 유지한다.

---

### Requirement: http_5xx / network / timeout 에러는 exponential backoff 공식을 적용한다
`errorKind`가 `"http_5xx"`, `"network"`, `"timeout"` 중 하나인 실패 보고에 대해 URLScheduler는 대상 row의 `fetch_error_count`(또는 `harvest_error_count`)를 1 증가시키고, `next_fetch_at`(또는 `next_harvest_at`)을 다음 공식으로 갱신해야 한다(SHALL):

```
delay      = 30s * 2^(error_count_after - 1)
jitter     = uniform[-0.1 * delay, +0.1 * delay]   (uniform 분포, 정규분포 아님)
next_*_at  = T_report + delay + jitter
```

여기서 `error_count_after`는 이번 실패를 반영한 후의 카운트 값(1..5)이며, `T_report`는 실패 보고 시점에 워커가 관측한 현재 시각이다. 공식은 단일 UPDATE로 반영되어야 한다(SHALL). `error_count_after` 증가와 `next_*_at` 갱신은 같은 row에서 찢어져서 관측되지 않아야 한다(SHALL NOT).

구현은 `error_count_after`가 5를 초과하지 않도록 보장해야 한다(SHALL). 결과적으로 최대 delay는 `30s * 2^4 = 480s` (8분)이며, `int64` nanosecond 범위 안에서 산술 overflow가 발생하지 않는다.

#### Scenario: 첫 fetch 실패 (non-4xx) backoff
- **WHEN** `fetch_error_count = 0`인 row에 대해 `"http_5xx"` errorKind로 `RecordFetchError`가 호출될 때 (호출 시각 = `T_report`)
- **THEN** `fetch_error_count`가 1로 증가하고, `next_fetch_at`이 `T_report + 30s ± 3s` (30s의 ±10% uniform jitter) 범위로 갱신된다.

#### Scenario: 두 번째 fetch 실패 backoff
- **WHEN** `fetch_error_count = 1`인 row에 대해 `"network"` errorKind로 `RecordFetchError`가 호출될 때 (호출 시각 = `T_report`)
- **THEN** `fetch_error_count`가 2가 되고, `next_fetch_at`이 `T_report + 60s ± 6s` 범위로 갱신된다.

#### Scenario: 네 번째 fetch 실패 backoff
- **WHEN** `fetch_error_count = 3`인 row에 대해 `"timeout"` errorKind로 `RecordFetchError`가 호출될 때 (호출 시각 = `T_report`)
- **THEN** `fetch_error_count`가 4가 되고, `next_fetch_at`이 `T_report + 240s ± 24s` 범위로 갱신된다.

#### Scenario: 다섯 번째 fetch 실패가 dead를 만든다
- **WHEN** `fetch_error_count = 4`인 row에 대해 `"http_5xx"` errorKind로 `RecordFetchError`가 호출될 때 (호출 시각 = `T_report`)
- **THEN** `fetch_error_count`가 5가 되고, `next_fetch_at`이 `T_report + 480s ± 48s` 범위로 갱신되지만, 이후 Pioneer claim 쿼리는 해당 row를 반환하지 않는다.

#### Scenario: jitter로 인해 동일 조건 보고의 next_fetch_at이 ±10% 경계 내 분산된다
- **WHEN** 동일 `error_count_after`와 동일 `T_report`를 가정한 N회(N ≥ 1000)의 실패 보고를 관찰할 때
- **THEN** 주 조건: 관측된 모든 `next_fetch_at` 값이 `[T_report + 0.9*delay, T_report + 1.1*delay]` 구간 내에 속한다 — 이 경계 조건이 uniform 분포를 강제한다(정규분포였다면 꼬리 샘플이 경계를 초과하여 이 조건을 위반). 보조 조건: 표본에 서로 다른 값이 2개 이상 관측된다(PRNG가 상수 출력이 아님을 확인하는 smoke check).

#### Scenario: harvest 측도 동일 공식 적용
- **WHEN** 동일한 `error_count_after` 값으로 `RecordHarvestError`의 delay 공식을 비교할 때
- **THEN** base(30s), 지수(`2^(n-1)`), jitter 비율(uniform ±10%) 모두 `RecordFetchError`와 동일하다.

---

### Requirement: 재시도 한도 5에 도달한 row는 dead로 취급되어 claim되지 않는다
`pioneer_frontier` row의 `fetch_error_count`가 5 이상이면 Pioneer claim 대상에서 영구적으로 제외되어야 한다(SHALL). `harvester_frontier` row의 `harvest_error_count`가 5 이상이면 Harvester claim 대상에서 영구적으로 제외되어야 한다(SHALL).

dead 상태는 archived change `scheduler-frontier-table`이 도입하여 현재 `openspec/specs/scheduler` spec에 반영된 partial index — pioneer 측은 `fetch_error_count < 5` 조건, harvester 측은 `harvested_at IS NULL AND harvest_error_count < 5` 조건 — 가 자동으로 제외하는 것으로 성립한다. 정확한 partial index 정의는 `openspec/specs/scheduler` spec이 단일 진실 원천(Single Source of Truth)이며 본 capability는 해당 index의 `… < 5` 조건에만 의존한다. 별도 `is_dead` boolean 컬럼이나 별도 상태 플래그를 도입해서는 안 된다(SHALL NOT).

URLScheduler는 dead 상태에 도달한 row에 대해 별도의 cleanup(삭제·아카이브)을 수행하지 않아야 한다(SHALL NOT) — cleanup은 본 capability의 책임이 아니다.

#### Scenario: dead row는 next_fetch_at이 도래해도 claim되지 않는다
- **WHEN** `fetch_error_count = 5`이고 `next_fetch_at <= now()`인 row가 존재할 때
- **THEN** Pioneer가 claim을 시도해도 해당 row는 반환되지 않는다(partial index가 `fetch_error_count < 5` 조건을 요구하므로 해당 row는 index에서 제외되어 있음).

#### Scenario: harvest 측도 동일하게 동작
- **WHEN** `harvest_error_count = 5`인 row가 존재할 때
- **THEN** Harvester claim 쿼리는 해당 row를 반환하지 않는다.

#### Scenario: 별도 is_dead 컬럼 부재
- **WHEN** `pioneer_frontier` / `harvester_frontier` 스키마를 확인할 때
- **THEN** `is_dead`나 유사한 boolean 컬럼이 존재하지 않으며, dead 판정은 `fetch_error_count >= 5` / `harvest_error_count >= 5` 조건만으로 이루어진다.

#### Scenario: dead row는 frontier에서 삭제되지 않는다
- **WHEN** row가 dead 상태에 도달한 직후 frontier 테이블을 조회할 때
- **THEN** 해당 row가 여전히 테이블에 존재하며, scheduler는 해당 row를 자동으로 삭제하거나 다른 테이블로 이동시키지 않는다.

---

### Requirement: 성공 시 error_count reset은 SetStatus 책임이다
본 capability의 `RecordFetchError` / `RecordHarvestError`는 **실패 경로만** 다루어야 한다(SHALL). `fetch_error_count = 0` / `harvest_error_count = 0` reset은 `scheduler-claim-api` capability가 정의하는 `SetStatus`의 책임이며, 본 capability 메서드는 reset 로직을 중복 구현해서는 안 된다(SHALL NOT). (참조: `scheduler-claim-api` spec의 `SetStatus` 요구사항.)

#### Scenario: RecordFetchError는 fetch_error_count를 감소시키지 않는다
- **WHEN** `fetch_error_count = 3`인 row에 대해 임의의 enum errorKind로 `RecordFetchError`가 호출될 때
- **THEN** 호출 직후 조회된 `fetch_error_count`는 4(`+= 1`) 또는 5(`"http_4xx"`) 둘 중 하나이며, 0 또는 3 이하의 값으로 감소하지 않는다.

#### Scenario: 성공 시 reset은 SetStatus가 담당
- **WHEN** Pioneer가 fetch 성공을 보고할 때
- **THEN** 호출되는 메서드는 `SetStatus(key, "fetched")`(signature는 `scheduler-claim-api` 참조)이며, 이 호출이 base `openspec/specs/scheduler` spec(archived `scheduler-frontier-table`이 도입한 "Pioneer fetch 성공 시" 요구사항)에 따라 `fetch_error_count = 0`과 `last_fetched_at = now()`를 갱신한다. 본 capability의 메서드는 이 경로에 관여하지 않는다.

---

### Requirement: 실패 보고 경로 외부에서 backoff 컬럼을 직접 수정하지 않는다
**런타임 애플리케이션 코드 경로**(Pioneer/Harvester 및 기타 서버 프로세스)는 `fetch_error_count`, `next_fetch_at`, `harvest_error_count`, `next_harvest_at` 컬럼을 `RecordFetchError` / `RecordHarvestError` / `SetStatus`(claim-api) 외부에서 직접 UPDATE해서는 안 된다(SHALL NOT). 한 번의 실패 보고는 단일 트랜잭션 안에서 일관되게 반영되어야 한다(SHALL).

본 제약은 **운영자의 수동 개입**(psql 세션으로 dead row를 수동 재활성화하는 운영 절차 등)에는 적용되지 않는다. 수동 개입은 런타임 코드 경로가 아니며, 운영 가이드에 기술된 절차를 통해서만 수행된다.

#### Scenario: 실패 보고 한 번의 호출이 단일 UPDATE로 반영된다
- **WHEN** 워커가 fetch 실패를 한 번 scheduler에 보고할 때
- **THEN** 해당 row의 `fetch_error_count`와 `next_fetch_at`이 단일 UPDATE(단일 트랜잭션) 안에서 함께 갱신된다.

#### Scenario: 실패 보고 후 row의 상태는 공식과 정확히 일치한다
- **WHEN** 워커가 한 번의 실패 보고(단일 `RecordFetchError` 또는 `RecordHarvestError` 호출)를 수행한 직후 해당 row를 조회할 때
- **THEN** `fetch_error_count`(또는 `harvest_error_count`)와 `next_fetch_at`(또는 `next_harvest_at`)이 본 spec의 공식과 정확히 일치하는 값을 보이며, 한쪽만 갱신되고 다른 쪽이 이전 값인 중간 상태는 관측되지 않는다.

---

### Requirement: 실패 보고는 last_updated_at을 현재 시각으로 갱신한다
`RecordFetchError` / `RecordHarvestError`는 실패를 반영하는 단일 UPDATE에서 대상 row의 `last_updated_at`을 현재 시각으로 갱신해야 한다(SHALL). 이는 4xx 즉시 dead 경로와 non-4xx backoff 경로 모두에 적용된다(SHALL). 운영 디버깅에서 "마지막으로 상태가 변한 시각"을 신뢰할 수 있도록 하기 위함이다.

#### Scenario: non-4xx 경로에서 last_updated_at 갱신
- **WHEN** `"http_5xx"` errorKind로 `RecordFetchError`가 호출될 때
- **THEN** 같은 UPDATE에서 해당 row의 `last_updated_at`이 호출 시각으로 갱신된다.

#### Scenario: 4xx 경로에서도 last_updated_at 갱신
- **WHEN** `"http_4xx"` errorKind로 `RecordFetchError`가 호출될 때
- **THEN** 같은 UPDATE에서 해당 row의 `last_updated_at`이 호출 시각으로 갱신된다(`fetch_error_count = 5` 설정과 함께).

---

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

---

### Requirement: URLScheduler는 EnqueueHarvester(url, snapshotKey) 메서드를 제공한다
`URLScheduler` 인터페이스는 다음 메서드를 추가로 제공해야 한다(SHALL):

- `EnqueueHarvester(url string, snapshotKey string) error` — `url`을 `harvester_frontier`에 UPSERT하고, 동일 호출에서 `snapshot_key` 컬럼을 `snapshotKey`로 세팅한다.

본 메서드는 baseline의 `Enqueue(QueueHarvester, urls...)`와 병행 제공되며, 서로 다른 두 가지 호출 상황을 분리한다.
- `Enqueue(QueueHarvester, urls...)`는 URL만 전달하는 기존 경로로, `snapshot_key`를 건드리지 않는다(baseline 규약 유지).
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

