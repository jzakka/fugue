## Context

`apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue` 의사코드는 Pioneer/Harvester 두 컴포넌트가 동일한 큐를 공유하면서 서로 다른 의미의 dequeue("not-visited" vs "not-parsed")를 호출하는 구조를 제안한다. 코드 주석에는 다음과 같은 핵심 의미가 평문으로만 남아 있다:

```
// block if pq is empty
// pq must guarantee that linearizable.
// status is scoring feature. not filtering criteria
```

또한 레거시 구현인 `apps/api/internal/bot/priority_queue.go`(인메모리 max-heap)는 단일 프로세스 안에서만 동작하며, 복수 워커 배포 시 동일 URL 중복 dequeue를 막지 못한다. `scheduler-frontier-table` change가 영속 저장소(`pioneer_frontier`, `harvester_frontier`, `harvester_frontier_pins`)와 partial index를 마련했지만, 그 위에 얹힐 **Go interface 계약과 claim 쿼리 패턴**은 정의되지 않았다.

본 change는 이 contract를 확정하여 Pioneer/Harvester 호출부 마이그레이션(후속 change들)이 안정된 API에 의존할 수 있게 한다. `DECISIONS.md` §2(Queue Split / Fanout), §3(Scheduler API) 결정을 그대로 반영한다.

## Goals / Non-Goals

**Goals:**
- `URLScheduler` Go interface 시그니처를 확정한다 (`Enqueue`, `Dequeue`, `SetStatus`, `RecordFetchError`, `RecordHarvestError`).
- `QueueType` enum(`pioneer`, `harvester`)을 도입하여 Dequeue의 대상 테이블을 결정한다.
- `Dequeue`의 두 핵심 의미 — **linearizability**(워커 간 동일 row 중복 claim 금지)와 **block-on-empty**(빈 큐/host throttle 동일 1초 폴링) — 를 구체적 구현 패턴(`SELECT ... FOR UPDATE SKIP LOCKED`, `time.Sleep(1s)`)으로 못 박는다.
- Claim 프로토콜을 host token bucket과 통합한다: 상위 N 후보 → `HostRateLimiter.Allow(host)` → 첫 통과 row claim.
- in-flight marker로 `next_fetch_at` / `next_harvest_at` 컬럼을 재활용하고 lease timeout은 base scheduler spec과 일치하여 10분으로 고정한다.
- `SetStatus`의 status enum(4종)과 `"harvested"` 시 `harvester_frontier_pins` INSERT 책임을 정의한다.
- `RecordFetchError`/`RecordHarvestError`를 별도 메서드로 분리하고 errorKind enum(4종)과 4xx 즉시 dead 규칙을 정의한다.

**Non-Goals:**
- 두 frontier 테이블의 스키마, 인덱스, 컬럼 nullable 정책 → `scheduler-frontier-table`.
- `next_fetch_at` / `next_harvest_at` backoff 공식 → `scheduler-retry-backoff`.
- `HostRateLimiter` 자체 구현(rate.Limiter 기반, SetHostRate 등) → `scheduler-host-token-bucket`.
- Pioneer/Harvester 호출부 코드를 실제로 `URLScheduler`로 교체하는 작업 → 후속 change(`harvester-scheduler-consumer` 등).
- `apps/api/internal/bot/priority_queue.go`, `bfs_queue.go` 실제 삭제(호출부 마이그레이션 완료 후 별도 정리 change).
- BFS depth 추적, MaxDepth/MaxNodesPerSite 같은 정책.

## Decisions

### Decision 1: `URLScheduler` interface 시그니처

```go
type QueueType string
type Status string
type ErrorKind string

const (
    QueuePioneer   QueueType = "pioneer"
    QueueHarvester QueueType = "harvester"
)

type URLScheduler interface {
    Enqueue(queueType QueueType, urls ...string) error
    Dequeue(queueType QueueType) (url string, err error)
    SetStatus(key string, status Status, pinIDs []uuid.UUID) error
    RecordFetchError(key string, errorKind ErrorKind) error
    RecordHarvestError(key string, errorKind ErrorKind) error
}
```

`Status`/`ErrorKind`를 `string` alias로 둔 것은 `QueueType`과 동일한 이유: 인라인 문자열 오기입을 컴파일 단계에서 잡기 위함이다. 상수 값 자체는 여전히 `"fetched"` 등 문자열이어서 SQL 바인딩·JSON 로깅에 지장이 없다.

**근거**:
- 의사코드의 `URLPriorityQueue`를 그대로 계승하되, "priority queue"라는 이름은 자료구조 종류를 노출하므로 더 일반적인 `URLScheduler`로 rename. 실제 구현은 인메모리 heap이 아닌 Postgres index scan이며, 향후 다른 저장소로 교체될 여지를 남긴다.
- `queryCondition` 추상은 폐기하고 `QueueType` enum으로 단순화. Pioneer/Harvester 두 경로만 지원하면 되는 현 MVP 요구사항에 충분하며, 조건 조립 라이브러리(squirrel 등)에 의존할 필요가 없다.
- `SetStatus`와 `RecordFetchError`/`RecordHarvestError`를 분리. SetStatus는 "결과 마킹"(완료 또는 실패 사실 기록), RecordXxxError는 "backoff·error_count 누적"으로 관심사가 다르다. Consumer는 실패 시 둘 다 호출.
- `Enqueue`는 `queueType`을 명시적으로 받아 Pioneer가 새 링크를 `pioneer_frontier`에, fetch 완료본을 `harvester_frontier`에 fanout하는 구조(DECISIONS §2)를 반영.

**대안**:
- (A) `queryCondition` 추상 유지 — 유연하지만 현재 두 경로만 있어 과설계. QueueType enum이 단순.
- (B) `SetStatus` 하나로 에러 backoff까지 처리 — 시그니처가 비대해지고 status별 파라미터(errorKind) 혼합이 지저분. 분리 채택.

### Decision 2: `Dequeue` linearizability — `FOR UPDATE SKIP LOCKED`

**선택**: Postgres `SELECT ... FOR UPDATE SKIP LOCKED` 패턴으로 claim 트랜잭션을 구성한다.

```sql
-- Pioneer claim 예시 (design.md 내부 참고용)
BEGIN;
SELECT id, url, host
  FROM pioneer_frontier
 WHERE fetch_error_count < 5
   AND next_fetch_at <= now()
 ORDER BY score DESC, next_fetch_at ASC
 LIMIT $1  -- SCHEDULER_CLAIM_CANDIDATE_N (default 1)
   FOR UPDATE SKIP LOCKED;
-- application: 각 row host에 HostRateLimiter.Allow(host) 호출, 첫 true row에 대해:
UPDATE pioneer_frontier
   SET next_fetch_at = now() + interval '10 minutes',
       last_updated_at = now()
 WHERE id = $winner_id;
COMMIT;
```

**근거**:
- 워커 N개가 동시에 같은 partial index를 스캔하더라도 `FOR UPDATE`가 행 락을 잡고 `SKIP LOCKED`가 다른 워커의 잠긴 row를 건너뛰므로, 동일 row가 두 워커에 동시에 dequeue되지 않는다.
- `FOR UPDATE`는 트랜잭션 종료(정상 commit 또는 connection close)에 자동 해제되어 워커 크래시 시 리커버리가 자연스럽다.

**대안**:
- (A) advisory lock — lock key 충돌/디버깅 어려움.
- (B) 별도 `claim_token UUID` 컬럼 — partial index 조건 추가와 만료 처리 필요. `FOR UPDATE SKIP LOCKED`가 가장 단순.

### Decision 3: `Dequeue` block-on-empty — 1초 고정 sleep

**선택**: claim 시도가 (a) 0 row 반환 또는 (b) host throttle로 모두 실패할 경우, 두 경우 모두 `time.Sleep(1 * time.Second)` 후 재시도한다. 상한 없는 무한 루프.

```go
for {
    url, ok, err := s.tryClaim(queueType)
    if err != nil {
        return "", err
    }
    if ok {
        return url, nil
    }
    time.Sleep(1 * time.Second) // 빈 큐/throttle 동일 처리
}
```

**근거**:
- `DECISIONS.md §3` 결정. 단순하고 외부 의존(LISTEN/NOTIFY, Redis pubsub) 없음.
- 빈 큐와 host throttle를 구분하지 않는 이유: 둘 다 "지금 당장 claim 가능한 작업이 없다"는 동일 상태이며, 대기 전략을 분기할 운영상 이유가 없다.
- 워커 N개 × 1 QPS의 idle SELECT가 발생하지만 partial index 스캔이라 비용은 작다.

**대안**:
- (A) LISTEN/NOTIFY — connection 점유, 메시지 유실 가능. 후속 옵션.
- (B) exponential backoff — 단순성 우선으로 기각.

### Decision 4: Claim 프로토콜 — top N 후보 + HostRateLimiter.Allow

**선택**: Dequeue의 단일 시도(`tryClaim`)는 다음 순서로 동작한다:

1. partial index ORDER BY로 상위 **N rows**를 `FOR UPDATE SKIP LOCKED`로 잠근다.
   - N은 환경변수 `SCHEDULER_CLAIM_CANDIDATE_N`, **default 1**.
2. 잠긴 각 row에 대해 host 컬럼 값으로 `HostRateLimiter.Allow(host)`를 호출한다.
3. 처음 `true`를 반환한 row를 winner로 claim:
   - Pioneer: `UPDATE next_fetch_at = now() + interval '10 minutes'`
   - Harvester: `UPDATE next_harvest_at = now() + interval '10 minutes'`
   - `last_updated_at = now()` 동시 갱신
4. 트랜잭션 COMMIT → winner URL 반환.
5. 모든 후보가 false면 트랜잭션 ROLLBACK 후 호출자에게 "claim 실패"를 리턴 → Dequeue는 1초 sleep 후 재시도.

**근거**:
- `DECISIONS.md §3, §5, §10` 결정. Robots filter는 Enqueue 단계에서, host bucket은 Claim 단계에서 체크하여 책임을 분리한다.
- Default N=1은 대다수 워커 배포에서 충분. 특정 호스트가 partial index head를 계속 점유하는 starvation이 관측되면 N을 키워 후보 다양성을 확보한다.
- `HostRateLimiter.Allow(host)`는 `scheduler-host-token-bucket` change의 인메모리 rate.Limiter를 호출. 본 change는 시그니처 `Allow(host string) bool` 만 가정.

**대안**:
- (A) Enqueue 단계에서 host throttle 체크 — enqueue 시점의 토큰 상태가 claim 시점의 상태와 다를 수 있어 의미가 없다. Claim 단계가 옳음.
- (B) 큰 N + 정교한 스코어링 — 단순성 우선으로 기각. 필요 시 env로 조정.

### Decision 5: in-flight marker — `next_fetch_at` / `next_harvest_at` 재활용

**선택**: 별도 `claimed_at` 또는 `in_flight` 컬럼을 도입하지 않는다. 대신:
- Pioneer: claim 시 `next_fetch_at = now() + 10min` UPDATE. partial index 조건 `next_fetch_at <= now()` 때문에 즉시 제외된다.
- Harvester: claim 시 `next_harvest_at = now() + 10min` UPDATE. 마찬가지로 partial index에서 제외.
- **Lease timeout 10분 고정** (base scheduler spec과 일치). 워커가 10분 내에 SetStatus/RecordXxxError를 호출하지 않으면 lease 만료 → partial index에 다시 등장 → 다른 워커가 재claim. 후속 운영 관측 결과에 따라 env 노출이 필요해지면 별도 change에서 base spec과 함께 완화한다.

**근거**:
- `DECISIONS.md §3`. 스키마 변경을 피하고 기존 컬럼 의미를 확장. 10분은 fetch/harvest 정상 소요 시간의 수 배 마진.
- 성공 시 `SetStatus("fetched", nil)`이 `next_fetch_at = now() + 365 days`로 덮어쓰므로 lease 재계산 불필요(DECISIONS §8).

**대안**:
- (A) 별도 `claimed_at TIMESTAMPTZ` 컬럼 — 스키마 변경 필요, partial index 재정의 필요. 이득 대비 비용 큼.
- (B) advisory lock을 lease로 사용 — connection 종료 시점과 lease 의미가 얽힘.

**테스트 전략 — Clock 주입**:
lease 만료 후 자동 재claim 시나리오를 10분을 실제로 기다리지 않고 검증하기 위해, scheduler 패키지는 `scheduler-retry-backoff`가 이미 도입한 `Clock` interface(`Now() time.Time`)와 `RealClock()` 팩토리를 재사용한다. lease 만료 테스트는 두 가지 접근이 가능하며 본 change는 **(a)** 를 기본으로 한다:

(a) **Go 측 clock 주입**: `PGURLScheduler`(또는 동등 구현체)가 `Clock` 의존성을 갖고, `tryClaim`이 `next_*_at = s.clock.Now() + leaseDuration` 으로 계산해 바인딩한다. 테스트는 `fakeClock`을 10분 앞당긴 뒤 두 번째 Dequeue를 호출하여 재claim 여부를 확인한다. Postgres는 `now()`가 아니라 Go가 전달한 시각을 사용하므로 SQL 변경 없이 빠른 테스트가 가능하다.

(b) **DB row 시각 조작**: 대안으로 Postgres 세션 시각 설정을 건드리지 않고, `UPDATE ... SET next_*_at = now() - interval '11 minutes'` 같은 헬퍼 쿼리로 row의 `next_*_at` 컬럼 자체를 과거로 이동시켜 lease 만료를 시뮬레이션. 별도 DB 라운드트립이 필요해 (a)보다 느리고 테스트 간 race 위험이 있어 기본안은 아니다.

본 change의 tasks.md 4.10은 (a) 방식을 명시적으로 요구한다.

### Decision 6: `Enqueue` 의 upsert 동작

**선택**:
- `pioneer_frontier`: `INSERT ... ON CONFLICT (url_hash) DO NOTHING`.
- `harvester_frontier`: `DECISIONS §8` 규칙을 따르되, **본 change의 `Enqueue(queueType, urls...)` 시그니처는 URL 이외의 컨텍스트(특히 `snapshot_key`)를 전달할 방법이 없으므로**, 본 change는 `snapshot_key`를 건드리지 않는 UPSERT만 정의한다:

```sql
INSERT INTO harvester_frontier (normalized_url, url, url_hash, host, score)
VALUES (...)
ON CONFLICT (url_hash) DO UPDATE
  SET next_harvest_at = now(),
      harvest_error_count = 0,
      last_updated_at = now()
  WHERE harvester_frontier.harvested_at IS NULL;
```

즉 이미 harvest된 URL에 대한 재enqueue는 no-op, 아직 harvest되지 않은 URL에 대해서는 `next_harvest_at` · `harvest_error_count` 만 갱신한다.

`snapshot_key` 초기 설정과 재크롤 갱신은 본 change의 `URLScheduler` 범위 바깥이다. Pioneer가 pin URL을 발견하여 harvester_frontier에 최초 row를 생성할 때는 `snapshot_key`를 포함한 구조화된 enqueue 경로(후속 change `harvester-scheduler-consumer` / `harvester-pin-document`)를 사용해야 한다. 본 change의 Enqueue는 **최소 필드만 있는 URL 레벨 upsert**이다.

**근거**:
- `url_hash`에 unique constraint가 있어 DB 레벨 충돌은 발생. `ON CONFLICT`로 호출자에게 violation을 노출하지 않는다.
- Harvester는 재harvest하지 않는다는 DECISIONS §8 정책을 SQL 레벨에서 강제.
- 문자열 URL만 받는 Enqueue 시그니처에 snapshot_key 필드를 끼워넣지 않고, 구조화된 전달은 후속 change로 이관하여 본 change의 contract를 단순하게 유지.

**NOT NULL 컬럼 기본값** (base scheduler spec이 `depth`/`score`를 pioneer에서 NOT NULL로 요구):
- `depth`: URL만 받는 본 change의 Enqueue는 `depth = 0`을 기본값으로 사용한다. BFS depth 전파는 parent row의 depth를 아는 호출자에게만 가능하므로, 그 요구를 충족시키는 **구조화된 enqueue 경로**는 후속 change(`pioneer-scheduler-consumer` 등)에서 도입한다.
- `score`: 본 change는 `score = 0.0`을 기본값으로 사용한다. Score 계산(`scheduler-scoring-policy` 류)은 별도 change 범위.
- `host`: URL에서 파싱하여 자동 채운다(base spec의 host 저장 규칙 준수: 포트 제외, 대소문자 보존, `www.` 유지).

본 Enqueue 시그니처는 고정 기본값을 적용하는 low-friction 경로이며, 세밀한 depth/score/snapshot_key를 지정해야 하는 호출자는 후속 change의 구조화된 enqueue를 써야 한다.

### Decision 7: `SetStatus` 의 의미와 harvester_frontier_pins INSERT 책임

**선택**: `SetStatus(key string, status string, pinIDs []uuid.UUID) error`. `pinIDs`가 `[]uuid.UUID`인 이유는 `pins.id`가 UUID PRIMARY KEY이고 `harvester_frontier_pins.pin_id`도 UUID FK이기 때문(migration 000003, 000026).

- `status` enum (4종):
  - `"fetched"` — Pioneer 성공. `pioneer_frontier.last_fetched_at = now()`, `next_fetch_at = now() + 365 days`(DECISIONS §8), `fetch_error_count = 0` 리셋. `pinIDs`는 무시(nil 허용).
  - `"fetch_failed"` — Pioneer 실패 마킹. `next_fetch_at`은 RecordFetchError가 backoff로 다시 설정(분리). SetStatus는 마킹 의미만.
  - `"harvested"` — Harvester 성공. `harvester_frontier.harvested_at = now()` 갱신, `harvest_error_count = 0` 리셋, **동일 트랜잭션 내에서** `pinIDs` 각 요소에 대해 `INSERT INTO harvester_frontier_pins (frontier_id, pin_id) VALUES (..., ...)` 수행.
  - `"harvest_failed"` — Harvester 실패 마킹. `pinIDs`는 nil.

**pinIDs INSERT 책임**:
- `"harvested"` 호출 시 `harvester_frontier_pins` INSERT는 SetStatus 내부 책임. Consumer가 별도로 pins 테이블에 매핑 INSERT를 호출하지 않는다.
- 트랜잭션: `harvester_frontier` UPDATE와 `harvester_frontier_pins` INSERT는 동일 DB 트랜잭션에서 실행되어 원자성 보장.
- `pinIDs`가 비어 있으면(길이 0) INSERT 없이 `harvested_at`만 갱신.

**근거**:
- `DECISIONS.md §3`. Harvester consumer가 pin 생성 후 scheduler에 "결과 보고"만 하면 되도록 단순화.

### Decision 8: `RecordFetchError` / `RecordHarvestError` — errorKind enum 4종

**선택**: 
```go
// errorKind 값 (ErrorKind = string alias, Decision 1 참조)
const (
    ErrorHTTP4xx  ErrorKind = "http_4xx"
    ErrorHTTP5xx  ErrorKind = "http_5xx"
    ErrorNetwork  ErrorKind = "network"
    ErrorTimeout  ErrorKind = "timeout"
)
```

- `"http_4xx"`: 즉시 `fetch_error_count = 5` (Pioneer) 또는 `harvest_error_count = 5` (Harvester)로 설정 → 즉시 dead. backoff 공식 적용하지 않음.
- `"http_5xx"`, `"network"`, `"timeout"`: `error_count = error_count + 1`, `next_*_at`는 `scheduler-retry-backoff`의 공식으로 갱신.

**Consumer 호출 규약**: 실패 시 `SetStatus(key, "*_failed", nil)` 와 `RecordFetchError(key, errorKind)`(또는 Harvester 변형) 를 **둘 다** 호출. 성공 시에는 SetStatus만.

**근거**:
- `DECISIONS.md §3, §4`. 4xx는 재시도해도 결과가 같을 가능성이 압도적(링크 오타/삭제) → 즉시 dead. 5xx/network/timeout은 일시적 장애 가능성 → backoff 재시도.
- SetStatus와 분리함으로써 "결과 마킹"과 "backoff 계산" 책임을 명확히 구분. 성공 케이스는 RecordXxxError 경로를 탈 일이 없다.

### Decision 9: 본 change 범위에서 호출부를 교체하지 않는다

Pioneer/Harvester 코드(`internal/bot/pioneer/*.go`, `internal/bot/harvester/*.go`)의 실제 호출부 교체는 후속 change(`harvester-scheduler-consumer` 등)에서 진행한다. 본 change는 **interface 정의 + Postgres 구현체 + 단위 테스트** 까지만 포함하며, 예외적으로 `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue` 타입/호출 부분에 **deprecation 주석만** 추가한다 (타입 rename, 시그니처 변경, 호출 치환 없음).

**근거**:
- 호출부 마이그레이션은 Pioneer worker budget, Harvester pin_document 등 다른 진행 중 change와 결합도가 높아 별도 PR로 다루는 편이 안전.

## Risks / Trade-offs

- **폴링 idle load** — 워커 N개 × 1 QPS의 빈 SELECT 쿼리. partial index 스캔이라 비용은 작지만, 워커 수가 수십 개로 증가하면 모니터링 필요 → LISTEN/NOTIFY 또는 backoff sleep으로 교체 가능.
- **Lease 10분 고정** — 네트워크가 매우 느린 대형 페이지에서 fetch가 10분을 넘기면 lease 만료 후 중복 fetch 발생. 실측 기반으로 조정 필요할 경우 base spec과 함께 env 노출하는 후속 change 필요.
- **`SetStatus` string 기반** — typo 위험. 구현 시 package 레벨 const(`StatusFetched`, `StatusFetchFailed`, ...)와 검증 로직으로 방어.
- **`FOR UPDATE SKIP LOCKED` starvation** — partial index head row가 host throttle을 계속 맞으면 후순위 row만 처리될 수 있음. N>1로 후보 다양성 확보 가능.
- **Claim 트랜잭션 길이** — host throttle 체크를 트랜잭션 내부에서 하므로, `HostRateLimiter.Allow`는 O(1) 인메모리 호출이어야 한다(scheduler-host-token-bucket이 보장).

## Migration Plan

1. 본 change에서 `apps/api/internal/scheduler/` 패키지를 신설하고 `URLScheduler` interface와 Postgres 구현체를 추가. sqlc 쿼리(`enqueue_pioneer`, `enqueue_harvester`, `claim_pioneer`, `claim_harvester`, `set_status_*`, `insert_harvester_frontier_pins`, `record_fetch_error`, `record_harvest_error`)를 query 파일에 등록.
2. unit/integration 테스트 추가 — 두 워커 동시 dequeue 시 동일 row 미반환, 빈 큐/host throttle dequeue 시 1초 단위 폴링, upsert 멱등성, `"harvested"` 호출 시 `harvester_frontier_pins` INSERT 원자성, `http_4xx` errorKind 시 즉시 dead, partial index EXPLAIN 사용.
3. Pioneer/Harvester 호출부는 본 change에서 변경하지 않는다. 의사코드(`fuguebot_pseudo.go`)의 `URLPriorityQueue` 타입과 `Dequeue(string)` 호출에 deprecation 주석만 추가.
4. 호출부 마이그레이션은 후속 change(`harvester-scheduler-consumer`, Pioneer 측은 별도)에서 단계적으로 진행.
5. 롤백: 본 change만 단독 롤백 시 신규 패키지 제거. frontier 테이블은 그대로 유지(다른 change 소유).

## Open Questions

- 폴링 주기 1초가 운영 부하상 과도/부족한지 — 메트릭 수집 후 후속 change에서 튜닝.
- Lease 10분이 실제 fetch/harvest 소요 분포에 적합한지 — 관측 후 조정. 필요시 env 노출은 후속 change.
- `SCHEDULER_CLAIM_CANDIDATE_N` default 1이 starvation 관점에서 충분한지 — 프로덕션 partial index head 점유율 관측 후 조정.
- ~~`Dequeue`가 `context.Context`를 받지 않아 graceful shutdown이 어려움~~ — **결정**: 본 change의 interface는 context.Context를 받지 않는 Decision 1의 시그니처를 유지한다. 대신 spec Requirement의 "다섯 메서드를 정확히 가져야 한다(SHALL)"를 "다섯 메서드를 최소한 가져야 한다"로 완화하여, 후속 change에서 `DequeueCtx(ctx, queueType)` overload를 추가하는 것이 본 spec의 breaking 변경이 되지 않도록 한다. 구체적 shutdown 전략(인터럽트 채널, overload, 교체)은 호출부 마이그레이션 change에서 결정.
