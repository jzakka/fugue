## Context

현재 Pioneer는 `apps/api/internal/bot/priority_queue.go`의 `PriorityQueue`(인메모리 max-heap)와 `apps/api/internal/bot/bfs_queue.go`의 `BFSQueue`(레벨별 슬라이스)를 사용해 단일 프로세스 안에서만 BFS를 진행한다. Harvester는 별도 큐 없이 DB 노드 목록을 폴링한다. `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue` 의사코드는 두 컴포넌트가 동일 큐를 공유하고 status 인자로 필터링하는 구조를 제안하지만, 아직 구현되지 않았다.

운영 요구는 다음과 같다:
- Pioneer/Harvester를 **복수 프로세스**로 실행해도 동일 URL을 두 번 fetch하지 않아야 한다.
- 프로세스가 죽어도 frontier 상태가 보존되어야 한다.
- 사이트별 동시 요청 수 제어(host token bucket), 실패 백오프(retry backoff)는 후속 change에서 동일 테이블 위에 얹는다.

본 change는 **테이블 스키마만** 확정한다. claim 쿼리, backoff 산식, host bucket 정책은 별도 change에서 다룬다.

## Goals / Non-Goals

**Goals:**
- Pioneer/Harvester가 공유할 단일 frontier 테이블의 컬럼·인덱스·제약을 정의.
- Pioneer claim 경로(아직 fetch되지 않은 high-score URL)와 Harvester claim 경로(아직 pin이 만들어지지 않은 fetched URL)가 각각 partial index로 O(log n)에 처리되도록 설계.
- bot spec에서 인메모리 BFS를 전제로 한 requirement를 제거하여, 후속 change가 frontier 기반으로 자유롭게 재정의할 수 있게 한다.

**Non-Goals:**
- URLScheduler Go interface 정의 및 `URLPriorityQueue` rename → `scheduler-claim-api`.
- claim 쿼리, `SELECT ... FOR UPDATE SKIP LOCKED` 또는 advisory lock 선택 → `scheduler-claim-api`.
- `next_fetch_at` / `next_harvest_at` 산정 공식(exponential backoff 등) → `scheduler-retry-backoff`.
- host별 동시 요청 제어 → `scheduler-host-token-bucket`.
- 기존 `PriorityQueue`/`BFSQueue` 코드 삭제, `bot_graph_node` 와의 관계 정리.
- harvest 결과를 `pin_id`에 채우는 트랜잭션 로직.

## Decisions

### Decision 1: `status` enum 컬럼을 두지 않는다

**선택**: status 컬럼 없이 `last_fetched_at`, `pin_id`, `*_error_count`, `next_*_at` 4종 컬럼의 조합으로 모든 상태를 표현한다.

**대안**:
- (A) `status` enum (`pending`, `fetching`, `fetched`, `harvesting`, `harvested`, `failed`)
- (B) 본 change 채택안: 시간/카운터 컬럼 조합

**근거**:
- enum은 상태 전이 트랜잭션이 필요하고, "fetching 중인데 워커가 죽으면 리커버리"용 별도 timeout 컬럼이 또 필요해진다. 결국 시간 컬럼이 필수이므로 enum은 redundant.
- Pioneer/Harvester가 보는 "claimable" 조건이 서로 다르다 (Pioneer는 `last_fetched_at IS NULL`, Harvester는 `pin_id IS NULL AND last_fetched_at IS NOT NULL`). 각자 partial index로 직접 표현하는 편이 단순하다.
- 새 워크플로우(예: re-harvest, content refresh)가 추가될 때 enum을 ALTER하지 않고 컬럼 조합만 바꾸면 된다.

### Decision 2: Pioneer claim용 partial index

```sql
CREATE INDEX bot_frontier_pioneer_claimable_idx
  ON bot_frontier (score DESC, normalized_url)
  WHERE last_fetched_at IS NULL
    AND fetch_error_count < 5
    AND next_fetch_at <= now();
```

**근거**:
- Pioneer는 "한 번도 fetch 안 했고, retry quota 남았고, backoff 끝난 row 중 score 높은 것"을 원한다. 세 조건 모두 partial index의 WHERE에 박아 인덱스 크기를 최소화한다.
- `score DESC` 정렬로 ORDER BY가 인덱스만으로 충족.
- `now()` 비교는 partial index에 직접 들어갈 수 없으므로 인덱스 정의 시 `next_fetch_at`는 WHERE에 그대로 두고, 쿼리 측에서 `next_fetch_at <= now()` 조건이 인덱스를 사용할 수 있도록 정렬 키에 포함시키는 방법도 검토할 수 있다. 본 design은 partial WHERE에 시간 비교를 두지 않는 형태(`WHERE last_fetched_at IS NULL AND fetch_error_count < 5`)로 유지하고, `next_fetch_at <= now()`는 쿼리 시점 추가 필터로 처리하는 것을 권장한다. (Postgres partial index는 IMMUTABLE 표현식만 허용)

**보완**: 따라서 실제 인덱스는 다음과 같다.
```sql
CREATE INDEX bot_frontier_pioneer_claimable_idx
  ON bot_frontier (score DESC, next_fetch_at)
  WHERE last_fetched_at IS NULL
    AND fetch_error_count < 5;
```

### Decision 3: Harvester claim용 partial index

```sql
CREATE INDEX bot_frontier_harvester_claimable_idx
  ON bot_frontier (score DESC, next_harvest_at)
  WHERE pin_id IS NULL
    AND harvest_error_count < 5;
```

**근거**: Pioneer 인덱스와 동일한 논리. `pin_id IS NULL`이 "아직 harvest 안 됨"을 의미. Harvester는 fetch 완료(`last_fetched_at IS NOT NULL`)를 추가 쿼리 조건으로 본다 — 인덱스의 partial WHERE에 IS NOT NULL을 더 넣을지 여부는 사이트의 fetch→harvest 비율에 따라 후속 change에서 튜닝.

### Decision 4: `host` 컬럼은 별도 보조 인덱스

```sql
CREATE INDEX bot_frontier_host_idx ON bot_frontier (host);
```

**근거**: `scheduler-host-token-bucket` change에서 host별 inflight 카운트를 빠르게 집계하기 위함. URL에서 host를 매번 파싱하지 않도록 application 측에서 enqueue 시점에 채운다.

### Decision 5: `pin_id` nullable, FK는 두지 않는다 (또는 ON DELETE SET NULL)

**선택**: `pin_id BIGINT NULL`. FK 제약은 본 change에서는 두지 않고, 운영상 필요 시 후속 change에서 `ON DELETE SET NULL`로 추가.

**근거**: pin이 삭제(예: 중복 정리)되어도 frontier row가 함께 삭제되면 안 된다. frontier는 "어디까지 처리했나"의 기록이므로 보존되어야 한다.

### Decision 6: `last_updated_at` trigger vs application

**선택**: application 측에서 매 UPDATE 시 명시적으로 set. trigger는 두지 않는다.

**근거**: sqlc 기반 코드베이스 컨벤션과 일관. 디버깅 용이.

## Risks / Trade-offs

- **partial index의 `next_fetch_at <= now()` 미포함** → 백오프 대기 중인 row가 인덱스에는 들어 있지만 쿼리에서 제외됨. 카운터 < 5인 대기 row 수가 많으면 인덱스가 비대해질 수 있음 → 운영 모니터링 후 필요 시 인덱스 정의에 시간 컷오프(예: `next_fetch_at < '2099-01-01'`)를 더하거나, 영구 실패 row를 별도 archive 테이블로 분리.
- **status 컬럼 부재로 인한 디버깅 비용** → 운영자가 row 상태를 한눈에 보기 어려움 → SQL view (`bot_frontier_status_v`) 를 후속 change에서 제공 가능. 본 change 범위 외.
- **MaxNodesPerSite 동작 변경 보류** → bot spec에서 관련 requirement는 제거되지만, 새 frontier 기반 quota 동작이 정의되기 전까지는 운영상 사이트별 노드 수 폭발 위험이 잠시 존재. `scheduler-claim-api` change와 짧게 묶어 진행할 것.
- **stale edge 삭제 requirement 제거** → 동일하게 후속 change에서 frontier 기반으로 재정의될 때까지 stale edge가 남을 수 있음. 그래프 시각화에는 영향이 있으나, harvest 정확성에는 영향 없음.

## Migration Plan

1. 본 change에서 마이그레이션 스크립트로 `bot_frontier` 테이블과 4개 인덱스를 추가.
2. 기존 데이터 백필은 하지 않는다(동작이 아직 frontier로 옮겨가지 않으므로).
3. 후속 change `scheduler-claim-api`에서 `URLScheduler` interface와 enqueue/dequeue 구현을 추가하고, Pioneer/Harvester 호출부를 교체. 이 시점에 `priority_queue.go`, `bfs_queue.go` 삭제.
4. 롤백: 본 change만 단독으로 롤백 시 테이블 DROP. 사용처가 아직 없으므로 안전.

## Open Questions

- `score` 타입을 `INTEGER`로 할지 `DOUBLE PRECISION`으로 할지 — 현재 `QueueItem.Priority`가 int이므로 INTEGER로 두되, semantic priority modifier 도입 시 부동소수가 필요해지면 후속 change에서 ALTER.
- `depth`의 의미를 frontier 단계에서 유지할지, 그래프 노드에서만 유지할지 — 본 change는 frontier에 `depth INT NOT NULL DEFAULT 0`로 보존.
- `normalized_url`의 길이 제한 — Postgres `TEXT`로 두되 unique index 크기를 위해 hash 기반 보조 컬럼이 필요한지는 운영 후 결정.
