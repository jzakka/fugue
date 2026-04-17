## Context

`apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue` 의사코드는 Pioneer/Harvester 두 컴포넌트가 동일한 큐를 공유하면서 서로 다른 의미의 dequeue("not-visited" vs "not-parsed")를 호출하는 구조를 제안한다. 코드 주석에는 다음과 같은 핵심 의미가 평문으로만 남아 있다:

```
// block if pq is empty
// pq must guarantee that linearizable.
// status is scoring feature. not filtering criteria
// or status would be a query
```

또한 레거시 구현인 `apps/api/internal/bot/priority_queue.go`(인메모리 max-heap)는 단일 프로세스 안에서만 동작하며, 복수 워커 배포 시 동일 URL 중복 dequeue를 막지 못한다. `scheduler-frontier-table` change가 영속 저장소(`bot_frontier`)와 Pioneer/Harvester 각각의 partial index를 마련했지만, 그 위에 얹힐 **Go interface 계약과 claim 쿼리 패턴**은 정의되지 않았다.

본 change는 이 contract를 확정하여 Pioneer/Harvester 호출부 마이그레이션(후속 change들)이 안정된 API에 의존할 수 있게 한다.

## Goals / Non-Goals

**Goals:**
- `URLScheduler` Go interface 시그니처를 확정한다 (`Enqueue`, `Dequeue`, `SetStatus`).
- `Dequeue`의 두 핵심 의미 — **linearizability**(워커 간 동일 row 중복 claim 금지)와 **block-on-empty**(빈 큐에서 폴링 대기) — 를 구체적 구현 패턴(`SELECT ... FOR UPDATE SKIP LOCKED`, `time.Sleep(1s)` busy-wait)으로 못 박는다.
- `Dequeue`의 인자가 status 문자열이 아니라 `queryCondition` 임을 명시하여, Pioneer/Harvester가 동일 스케줄러를 다른 조건으로 호출할 수 있게 한다.
- `Enqueue`의 upsert 의미와 `SetStatus`의 보고 의미를 정의한다.

**Non-Goals:**
- `bot_frontier` 테이블 스키마, 인덱스, 컬럼 nullable 정책 → `scheduler-frontier-table`.
- `next_fetch_at` / `next_harvest_at` 산정 공식, error backoff 곡선 → `scheduler-retry-backoff`.
- host별 동시 요청 제어, token bucket → `scheduler-host-token-bucket`.
- Pioneer/Harvester 호출부 코드를 실제로 `URLScheduler`로 교체하는 작업 → 후속 change(`harvester-scheduler-consumer` 등).
- `apps/api/internal/bot/priority_queue.go`, `bfs_queue.go` 실제 삭제(호출부 마이그레이션 완료 후 별도 정리 change).
- queryCondition 객체의 구체 타입 설계 — 본 change는 "쿼리 빌더로 조립되는 조건"이라는 의미만 못 박고, 정확한 Go 타입(struct, builder, sqlc named arg 중 무엇)은 구현 task에서 결정.
- BFS depth 추적, MaxDepth/MaxNodesPerSite 같은 정책.

## Decisions

### Decision 1: `URLScheduler` interface 시그니처

```go
type URLScheduler interface {
    Enqueue(urls ...string)
    Dequeue(cond queryCondition) string
    SetStatus(key string, status string)
}
```

**근거**:
- 의사코드의 `URLPriorityQueue`를 그대로 계승하되, "priority queue"라는 이름은 자료구조 종류를 노출하므로 더 일반적인 `URLScheduler`로 rename. 실제 구현은 인메모리 heap이 아닌 Postgres index scan이며, 향후 다른 저장소로 교체될 여지를 남긴다.
- 메서드 수 3개로 최소화. 통계, 헬스체크 등은 본 interface 외부에서 처리.
- `Enqueue`는 가변인자로 다중 URL을 한 번에 받아 batch upsert를 가능하게 한다.
- 반환 타입을 `string`(URL)으로 둔 것은 의사코드와의 일관성. row 전체(`*FrontierRow`)가 필요한 호출부는 별도 조회 메서드를 사용하거나 후속 change에서 시그니처를 확장.

**대안**:
- (A) `Dequeue` 가 `(string, error)` 반환. 본 change는 의사코드 일관성 유지를 위해 `string`만 반환하고, 영구적 에러는 panic / 로그 / 컨텍스트 취소로 처리.
- (B) `Enqueue`가 score를 함께 받음(`Enqueue(items ...EnqueueItem)`). priority/score는 application 레이어가 enqueue 시점에 계산해 row에 넣어야 하므로 추후 시그니처 확장이 필요할 수 있음 — 본 change는 의사코드 호환을 우선시하고, score 인자 추가는 후속 change에서 하위호환적으로 처리.

### Decision 2: `Dequeue` linearizability — `FOR UPDATE SKIP LOCKED`

**선택**: Postgres `SELECT ... FOR UPDATE SKIP LOCKED` 패턴으로 dequeue 트랜잭션을 구성한다.

```sql
BEGIN;
SELECT id, url
  FROM bot_frontier
 WHERE <queryCondition>
 ORDER BY score DESC, next_fetch_at ASC
 LIMIT 1
   FOR UPDATE SKIP LOCKED;
-- application: row를 잠근 상태에서 work claim marker를 갱신하거나, 트랜잭션 종료 후 별도 UPDATE
COMMIT;
```

**근거**:
- 워커 N개가 동시에 같은 partial index를 스캔하더라도 `FOR UPDATE`가 행 락을 잡고 `SKIP LOCKED`가 다른 워커의 잠긴 row를 건너뛰므로, 동일 row가 두 워커에 동시에 dequeue되지 않는다 — linearizability 요구를 만족.
- advisory lock 대안은 lock key를 별도 관리해야 하고, 워커 죽으면 락 해제 타이밍이 모호. `FOR UPDATE`는 트랜잭션 종료 시 자동 해제.
- 트랜잭션 중간에 워커가 죽으면 Postgres가 connection 종료 시 락을 해제하므로, 다른 워커가 같은 row를 다시 claim 가능 — 자연스러운 리커버리.

**대안**:
- (A) Postgres advisory lock (`pg_try_advisory_xact_lock`) — lock key 충돌 가능성, 디버깅 어려움.
- (B) 별도 `claim_token UUID` 컬럼 + UPDATE ... WHERE claim_token IS NULL RETURNING — 가능하지만 partial index에 claim_token 조건을 더 넣어야 하고, 만료 처리(claim timeout)가 추가로 필요. `FOR UPDATE SKIP LOCKED`가 가장 단순.
- (C) Redis BRPOPLPUSH 류 — 영속성/일관성을 Postgres에 두기로 한 `scheduler-frontier-table`의 결정과 충돌.

### Decision 3: `Dequeue` block-on-empty — busy-wait + `time.Sleep(1s)`

**선택**: claim 쿼리가 0 row를 반환하면 1초 sleep 후 재시도. 이를 row가 잡힐 때까지 반복.

```go
for {
    if url, ok := s.tryClaim(cond); ok {
        return url
    }
    time.Sleep(1 * time.Second)
}
```

**근거**:
- 사용자 결정. 단순하고, 외부 의존(Postgres LISTEN/NOTIFY, Redis pubsub) 없이 동작.
- frontier가 빈 시간은 일반적으로 짧고(다른 워커가 곧 enqueue), 1초 latency는 운영적으로 허용 가능.
- 워커 N개 × 1 QPS의 idle SELECT가 발생하지만 partial index 스캔이라 부하는 미미. 운영상 부담이 커지면 후속 change에서 LISTEN/NOTIFY나 backoff sleep으로 교체 가능.

**대안**:
- (A) Postgres `LISTEN/NOTIFY` — Pioneer enqueue 시 NOTIFY → 대기 워커 즉시 wake-up. latency 0에 가깝지만 connection 점유, 메시지 유실 가능. 본 change에서는 채택하지 않고 추후 옵션으로 남김.
- (B) exponential backoff (1s → 2s → 4s … cap 30s) — idle 시 부하 절감. 단순성을 우선해 고정 1s 채택. 필요시 후속 change에서 도입.
- (C) `context.Context`로 cancel 가능하게 만들고 `select` 으로 `time.After(1s)` 와 `ctx.Done()` 대기 — 본 change의 interface가 `context.Context`를 받지 않으므로 적용 불가. 호출부가 종료를 원하면 외부에서 connection close로 처리. (시그니처 확장은 후속 change에서.)

### Decision 4: `queryCondition` 의 의미 — 쿼리 빌더 / 조건 객체

**선택**: `Dequeue`의 인자는 status 문자열이 아니라, **claim SQL의 WHERE 절을 조립하는 조건 객체**(`queryCondition`)이다. Pioneer/Harvester는 각자 자신이 쓸 partial index에 정확히 매칭되는 조건을 구성하여 호출한다.

- Pioneer 호출 시 조건 (개념):
  ```
  last_fetched_at IS NULL
  AND fetch_error_count < 5
  AND next_fetch_at <= now()
  ```
  → `bot_frontier_pioneer_claimable_idx` 사용.
- Harvester 호출 시 조건 (개념):
  ```
  pin_id IS NULL
  AND harvest_error_count < 5
  AND next_harvest_at <= now()
  ```
  → `bot_frontier_harvester_claimable_idx` 사용.

**근거**:
- 의사코드 주석 "status is scoring feature. not filtering criteria. or status would be a query" 를 그대로 형식화한 것.
- enum status(`pending`, `fetched` 등)로 분기하면 `scheduler-frontier-table`에서 의도적으로 제거한 status 컬럼을 사실상 application 측에 부활시키는 셈이 된다. 쿼리 조건 객체로 두면 `bot_frontier`의 컬럼 조합이 그대로 노출되어 일관성이 유지된다.
- 새 워크플로우(예: re-fetch, content refresh)가 추가될 때 새로운 조건 객체만 추가하면 되며, scheduler interface나 enum을 변경할 필요가 없다.

**구체 Go 타입의 결정은 본 change 범위 외**:
- 옵션 1: `type queryCondition struct { SQL string; Args []any }` — 가장 단순. 호출부가 raw SQL 일부를 들고 있어야 함.
- 옵션 2: `type queryCondition func(*sq.SelectBuilder) *sq.SelectBuilder` (squirrel 등) — 빌더 라이브러리 의존.
- 옵션 3: 미리 정의된 const(`PioneerClaimable`, `HarvesterClaimable`)로 좁혀두고, 신규 조건이 필요할 때만 추가 — 유연성과 안전성의 절충.

본 change는 옵션 3에 가깝게 운영하기를 권장하지만(스펙은 "쿼리 조건 객체" 추상으로만 못 박음), 실제 타입 선언은 구현 task에서 결정.

### Decision 5: `Enqueue` 의 upsert 동작

**선택**: `INSERT INTO bot_frontier (...) VALUES (...) ON CONFLICT (normalized_url) DO NOTHING`. score 갱신이 필요해지는 시점이 오면 `DO UPDATE SET score = GREATEST(bot_frontier.score, EXCLUDED.score)` 로 확장.

**근거**:
- `bot_frontier.normalized_url`에 unique constraint가 이미 있으므로(scheduler-frontier-table) DB 레벨 충돌은 발생. application 측에서 `ON CONFLICT DO NOTHING` 으로 silent하게 무시하여 호출자가 try/except를 반복할 필요 없게 한다.
- 같은 URL이 다른 부모로부터 여러 번 발견되어도 score는 처음 enqueue 시점 값으로 굳어지는 것이 본 change의 기본 동작. score 업데이트 시멘틱은 단순성이 깨지므로 후속 change에서 도입.

**대안**:
- (A) 호출부가 직접 unique violation을 catch — 코드 중복 발생.
- (B) `ON CONFLICT DO UPDATE SET score = ...` 즉시 적용 — 본 change는 score 동작을 명시하지 않음.

### Decision 6: `SetStatus` 의 의미와 인자 형식

**선택**: `SetStatus(key, status)` 는 frontier row를 식별하는 `key`(= `normalized_url`)에 대해, `status` 문자열이 표현하는 결과를 row에 반영한다. 본 change는 다음 두 status만 표준화한다:

- `"fetched"` — Pioneer가 fetch 성공 시. `last_fetched_at = now()` 갱신.
- `"fetch_failed"` — Pioneer가 fetch 실패 시. `fetch_error_count = fetch_error_count + 1`, `next_fetch_at`은 본 change에서는 단순히 `now()` 로 두고, backoff 곡선은 `scheduler-retry-backoff`에서 정의.
- `"harvested:<pin_id>"` — Harvester가 Pin 생성 성공 시. `pin_id`를 갱신.
- `"harvest_failed"` — Harvester가 harvest 실패 시. `harvest_error_count = harvest_error_count + 1`.

**근거**:
- 의사코드의 `SetStatus(Key(content), "pending")` 호출 형태를 가능한 한 보존.
- 실제로는 fetched/harvested의 부수 효과(저장 위치, pin id)를 함께 전달해야 하므로 단순 string 한 개로는 부족. `"harvested:<pin_id>"`처럼 prefix+payload 형태로 표준화. 추후 시그니처를 `SetStatus(key string, result Result)` 로 진화시키는 것이 자연스럽다.
- backoff 산식은 본 change에서 의도적으로 비워두며, `scheduler-retry-backoff`가 같은 SetStatus 호출 경로에 hooking.

**대안**:
- (A) `SetStatus(key string, result FetchResult | HarvestResult)` 인터페이스 분기 — 더 type-safe. 하지만 의사코드와의 일관성을 위해 본 change는 string 유지.
- (B) Pioneer/Harvester가 직접 SQL UPDATE — interface가 의미 없어짐. 거부.

### Decision 7: 본 change 범위에서 호출부를 교체하지 않는다

Pioneer/Harvester 코드(`internal/bot/pioneer/*.go`, `internal/bot/harvester/*.go`, 의사코드 포함)의 실제 호출부 교체는 후속 change(`harvester-scheduler-consumer` 등)에서 진행한다. 본 change는 **interface 정의 + Postgres 구현체 + 단위 테스트** 까지만 포함.

**근거**:
- 호출부 마이그레이션은 Pioneer worker budget, Harvester pin_document 등 다른 진행 중 change와 결합도가 높아 별도 PR로 다루는 편이 안전.
- contract 단독 머지로도 동작 변화 없음 → 작은 PR로 review 부담 최소화.

## Risks / Trade-offs

- **busy-wait의 idle load** — 워커 N개 × 1 QPS의 빈 SELECT 쿼리. partial index 스캔이라 비용은 작지만, 워커 수가 수십 개로 증가하면 무시 못 함 → 모니터링 후 LISTEN/NOTIFY 또는 backoff sleep으로 교체.
- **`Dequeue` 가 `context.Context`를 받지 않음** → 워커 종료 신호를 처리하기 어려움. 의사코드 호환 우선시. 후속 change에서 `DequeueCtx(ctx, cond)` 추가하는 식의 진화 필요.
- **`SetStatus` string 기반** → typo 위험, 새 status 추가 시 caller/callee 모두 수정. 본 change는 의사코드 호환을 우선시. 추후 `Result` 타입으로 진화 권장.
- **`queryCondition` 추상이 모호** — 구체 Go 타입을 본 change에서 못 박지 않아 구현 task에서 의견이 갈릴 수 있음 → tasks.md에 "옵션 3(미리 정의된 const)로 시작하고 필요 시 확장" 가이드 명시.
- **`FOR UPDATE SKIP LOCKED` 의 starvation** — partial index의 head row가 항상 락 충돌하면 후순위 row가 먼저 dequeue될 수 있음. 우선순위가 엄격한 ordering이 필요한 경우 문제. 본 시스템은 score 기반 best-effort 우선순위면 충분 → 허용.
- **트랜잭션 길이** — `FOR UPDATE` 후 application이 fetch까지 트랜잭션을 끌고 가면 다른 워커가 partial index에서 mass skip을 겪을 수 있음 → claim 트랜잭션과 fetch 작업을 분리: claim 트랜잭션 안에서는 row를 잠그고 즉시 "in-flight" marker(예: `last_fetched_at = now()` 또는 별도 컬럼)를 set한 뒤 `COMMIT`, 실제 fetch는 트랜잭션 외부에서 수행. 본 change의 partial index가 `last_fetched_at IS NULL` 조건이므로, 이 마킹만으로 다른 워커가 같은 row를 다시 claim하지 않는다.

## Migration Plan

1. 본 change에서 `apps/api/internal/scheduler/` 패키지를 신설하고 `URLScheduler` interface와 Postgres 구현체를 추가. sqlc 쿼리(`enqueue`, `claim_pioneer`, `claim_harvester`, `set_fetched`, `set_fetch_failed`, `set_harvested`, `set_harvest_failed`)를 query 파일에 등록.
2. unit/integration 테스트 추가 — 두 워커 동시 dequeue 시 동일 row 미반환, 빈 큐 dequeue 시 1초 단위 폴링, upsert 멱등성, SetStatus 후 partial index 제외 동작.
3. Pioneer/Harvester 호출부는 본 change에서 변경하지 않는다. 의사코드(`fuguebot_pseudo.go`)의 `URLPriorityQueue` 타입과 `Dequeue(string)` 호출만 코멘트로 deprecation 표시.
4. 호출부 마이그레이션은 후속 change(`harvester-scheduler-consumer`, Pioneer 측은 별도)에서 단계적으로 진행.
5. 롤백: 본 change만 단독 롤백 시 신규 패키지 제거. `bot_frontier` 테이블은 그대로 유지(다른 change 소유).

## Open Questions

- `queryCondition`의 구체 Go 타입을 (a) 미리 정의된 const, (b) struct, (c) builder closure 중 무엇으로 둘지 — 구현 task에서 PR 리뷰로 확정.
- `SetStatus("harvested:<pin_id>")` 의 string 인코딩이 적절한지, 아니면 처음부터 `SetStatus(key string, result Result)` 로 시그니처를 분리할지 — 본 change에서는 의사코드 호환을 위해 string 유지하지만, 후속 change에서 진화 필요.
- `Dequeue`가 `context.Context`를 받지 않아 graceful shutdown이 어려움 — 후속 change에서 `DequeueCtx` 도입할지 결정.
- busy-wait 폴링 주기 1초가 운영 부하상 과도/부족한지 — 메트릭 수집 후 후속 change에서 튜닝.
- claim 트랜잭션이 row 잠근 직후 in-flight marker로 어떤 컬럼을 사용할지(`last_fetched_at = now()` 즉시 set vs 별도 `claimed_at` 컬럼 신설) — 별도 컬럼 추가는 `scheduler-frontier-table`의 스펙을 손대므로 본 change에서는 `last_fetched_at`을 임시 marker로 재사용하는 안을 권장하되, 의미 충돌이 발생하면 `scheduler-frontier-table`에 컬럼 추가 후속 change로 분리.
