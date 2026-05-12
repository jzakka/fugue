## Context

현재 Pioneer는 `apps/api/internal/bot/priority_queue.go`의 `PriorityQueue`(인메모리 max-heap)와 `apps/api/internal/bot/bfs_queue.go`의 `BFSQueue`(레벨별 슬라이스)를 사용해 단일 프로세스 안에서만 BFS를 진행한다. Harvester는 별도 큐 없이 DB 노드 목록을 폴링한다. `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue` 의사코드는 두 컴포넌트가 동일 큐를 공유하고 status 인자로 필터링하는 구조를 제안하지만, 아직 구현되지 않았다.

운영 요구는 다음과 같다:
- Pioneer/Harvester를 **복수 프로세스**로 실행해도 동일 URL을 두 번 fetch하지 않아야 한다.
- 프로세스가 죽어도 frontier 상태가 보존되고, 크래시된 워커가 잡고 있던 row가 자동 회수되어야 한다.
- 사이트별 동시 요청 수 제어(host token bucket), 실패 백오프(retry backoff)는 후속 change에서 동일 테이블 위에 얹는다.

본 change는 **테이블 스키마만** 확정한다. claim 쿼리, backoff 산식, host bucket 정책은 별도 change에서 다룬다.

## Goals / Non-Goals

**Goals:**
- Pioneer 큐와 Harvester 큐의 컬럼·인덱스·제약을 각각 정의.
- Pioneer claim 경로와 Harvester claim 경로가 각각 partial index로 O(log n)에 처리되도록 설계.
- `next_*_at` 컬럼이 "다음 실행 시각"과 "in-flight lease marker" 두 용도를 겸하도록 규약 확정.
- Harvester 결과(Pin)와 frontier row 사이의 1:N 관계를 조인 테이블로 표현.
- bot spec에서 인메모리 BFS를 전제로 한 requirement를 제거하여, 본 change의 frontier 테이블이 이를 대체함을 spec 레벨에서 확정.

**Non-Goals:**
- URLScheduler Go interface 정의 및 `URLPriorityQueue` rename → `scheduler-claim-api`.
- claim 쿼리, `SELECT ... FOR UPDATE SKIP LOCKED` 채택 → `scheduler-claim-api`.
- `next_fetch_at` / `next_harvest_at` 산정 공식(exponential backoff 등) → `scheduler-retry-backoff`.
- host별 동시 요청 제어 → `scheduler-host-token-bucket`.
- 기존 `PriorityQueue`/`BFSQueue` 코드 삭제.
- Pioneer→Harvester fanout 시점의 트랜잭션 경계(원자성) 구체화는 `scheduler-claim-api`에서.

## Decisions

### Decision 1: Pioneer 큐와 Harvester 큐를 독립된 두 테이블로 분리한다

**선택**: `pioneer_frontier`, `harvester_frontier` 두 테이블로 분리. 단일 테이블 + `queue_type` enum 방식은 채택하지 않는다.

**근거**:
- Pioneer fetch와 Harvester harvest는 입력 URL이 다르다. Pioneer fetch의 결과는 (a) 새 링크 N개(→ `pioneer_frontier`), (b) 원본 URL 1개 + snapshot_key(→ `harvester_frontier`)로 **fanout**된다. 한 큐 안에 두 종류 row가 섞이면 claim 조건마다 `queue_type` 필터가 붙어 partial index가 두 배로 증가한다. (Context L3에서 언급한 `URLPriorityQueue` 의사코드의 "status 인자로 필터링" 방식도 같은 형태의 단일 저장소 + 구분 컬럼 변형으로 본 대안에 포함되어 함께 기각된다.)
- 재크롤/재harvest 정책이 다르다. Pioneer는 성공 시 `next_fetch_at = now() + 365 days`로 스케줄 재조정 (재크롤). Harvester는 재harvest를 하지 않고 `harvested_at IS NULL`에서만 동작 (DECISIONS §8).
- 에러 카운터 의미가 다르다. `fetch_error_count`(HTTP 실패)와 `harvest_error_count`(파싱/스크립트 실패)는 완전히 다른 원인으로 증가한다. 한 컬럼으로 묶으면 의미가 흐려진다.
- 조인 테이블(`harvester_frontier_pins`)은 Harvester만의 출력이다. Pioneer쪽에 이 FK를 끌어올 이유가 없다.

**대안 (기각)**:
- 단일 테이블 + `queue_type IN ('pioneer', 'harvester')` — 위 사유로 기각.

### Decision 2: 컬럼 정의 (DECISIONS §1 그대로)

#### `pioneer_frontier`

| 컬럼 | 타입 | NULL | DEFAULT |
|------|------|------|---------|
| `id` | `BIGSERIAL PRIMARY KEY` | NOT NULL | — |
| `normalized_url` | `TEXT` | NOT NULL | — |
| `url` | `TEXT` | NOT NULL | — |
| `url_hash` | `BYTEA` (sha256, 32 bytes) | NOT NULL | — |
| `host` | `TEXT` | NOT NULL | — |
| `depth` | `INTEGER` | NOT NULL | `0` |
| `score` | `DOUBLE PRECISION` (0.0~1.0) | NOT NULL | `0` |
| `last_fetched_at` | `TIMESTAMPTZ` | NULL | `NULL` |
| `next_fetch_at` | `TIMESTAMPTZ` | NOT NULL | `now()` |
| `fetch_error_count` | `INTEGER` | NOT NULL | `0` |
| `last_updated_at` | `TIMESTAMPTZ` | NOT NULL | `now()` |

- `UNIQUE(url_hash)`.
- Partial index:
  ```sql
  CREATE INDEX pioneer_frontier_claimable_idx
    ON pioneer_frontier (score DESC, next_fetch_at ASC)
    WHERE fetch_error_count < 5;
  ```

#### `harvester_frontier`

| 컬럼 | 타입 | NULL | DEFAULT |
|------|------|------|---------|
| `id` | `BIGSERIAL PRIMARY KEY` | NOT NULL | — |
| `normalized_url` | `TEXT` | NOT NULL | — |
| `url` | `TEXT` | NOT NULL | — |
| `url_hash` | `BYTEA` | NOT NULL | — |
| `host` | `TEXT` | NOT NULL | — |
| `snapshot_key` | `TEXT` | NULL | `NULL` |
| `score` | `DOUBLE PRECISION` | NOT NULL | `0` |
| `harvested_at` | `TIMESTAMPTZ` | NULL | `NULL` |
| `next_harvest_at` | `TIMESTAMPTZ` | NOT NULL | `now()` |
| `harvest_error_count` | `INTEGER` | NOT NULL | `0` |
| `last_updated_at` | `TIMESTAMPTZ` | NOT NULL | `now()` |

- `UNIQUE(url_hash)`.
- Partial index:
  ```sql
  CREATE INDEX harvester_frontier_claimable_idx
    ON harvester_frontier (score DESC, next_harvest_at ASC)
    WHERE harvested_at IS NULL
      AND harvest_error_count < 5;
  ```

#### `harvester_frontier_pins` (1:N 조인)

```sql
CREATE TABLE harvester_frontier_pins (
  frontier_id BIGINT NOT NULL REFERENCES harvester_frontier(id) ON DELETE CASCADE,
  pin_id      UUID   NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
  PRIMARY KEY (frontier_id, pin_id)
);
```

기존 `pins.id`가 `UUID`이므로 `pin_id`는 UUID로 맞춘다. `harvester_frontier.id`는 새로 추가되는 BIGSERIAL이며, 조인 테이블은 두 타입을 그대로 이어 받는다.

**근거**: ScriptAdapter의 N→1 규칙(정본 RawItem 1개 + 추가 media_candidates)과는 별개로, 한 frontier row가 시간에 걸쳐 여러 Pin을 생산할 수 있으며(예: 향후 재파싱), Pin은 다른 관점에서 여러 frontier와 엮일 수도 있다. Pin과 frontier의 결합도를 낮추기 위해 조인 테이블로 분리. `ON DELETE CASCADE`로 양쪽 삭제 시 조인 row도 자동 제거.

### Decision 3: `url_hash BYTEA` (sha256 32바이트, CHECK로 강제) + `sha256(normalized_url)`로 unique 제약

**선택**: 두 frontier 테이블 모두 unique 키는 `url_hash` (32바이트 BYTEA, sha256 raw). `normalized_url` 자체에 unique를 걸지 않는다.

**근거**:
- Postgres B-tree 인덱스는 행당 ~2700 bytes 이하를 요구. `normalized_url`이 매우 긴 query-string을 포함하면 unique index 생성 단계에서 실패할 수 있다. sha256 해시는 길이 무관하게 32바이트로 고정.
- hex 문자열(64자 TEXT)이 아닌 `BYTEA`로 저장하여 인덱스 크기를 절반으로 줄인다.
- 해시 충돌(sha256 2^128)은 운영상 무시 가능.
- 애플리케이션은 enqueue 시 `sha256(normalized_url)`을 계산하여 `url_hash` 컬럼에 바로 넣는다. DB 측 생성 컬럼/trigger는 두지 않는다(컨벤션).
- DB 수준 안전망으로 `CHECK (octet_length(url_hash) = 32)` 길이 제약을 두 frontier 테이블에 추가하여, 잘못된 길이의 값이 저장되는 사고를 차단한다. trigger/생성 컬럼이 아니므로 컨벤션을 해치지 않는다.
- CHECK 제약명은 명시적으로 고정한다: `pioneer_frontier_url_hash_len_chk`, `harvester_frontier_url_hash_len_chk`. 운영에서 제약 drop/alter 스크립트를 이름 기반으로 작성할 수 있도록 자동 생성명(`_check` 접미사)에 의존하지 않는다.

### Decision 4: `next_fetch_at` / `next_harvest_at`을 in-flight marker로 겸용한다

**선택**: 별도의 `claimed_at` / `lease_until` 컬럼을 두지 않는다. claim 시점에 `next_fetch_at` (또는 `next_harvest_at`)을 `now() + 10 minutes`로 UPDATE하여 lease marker로 사용한다.

**근거**:
- partial index의 정렬 키가 `next_*_at ASC`이므로, claim 시 미래로 밀어두면 같은 row가 상위로 다시 올라오지 않는다(다른 워커의 재-claim 방지).
- 워커 크래시 시 10분 후 자연히 index 상위로 복귀하여 다른 워커가 회수할 수 있다 (`SELECT FOR UPDATE SKIP LOCKED` 기반 구현에서 row lock은 트랜잭션 종료 시 해제되므로 별도의 cleanup job이 불필요).
- 성공/실패 시 `next_*_at`을 실제 다음 스케줄 시각으로 재갱신하므로 컬럼 의미가 "다음 처리 가능 시각"으로 자연스럽게 귀결.
- Lease timeout 값 10분은 DECISIONS §3에서 확정. Harvester의 HTML 파싱·이미지 캐싱 최대 시간을 여유 있게 덮는다.

**트레이드오프**: 컬럼 의미가 "다음 실행 시각"과 "현재 lease 종료 시각" 두 가지로 중의적이다. 스펙과 쿼리에 "in-flight인 row는 `next_*_at > now()`"라는 규약을 명문화하여 보완.

### Decision 5: `host` 컬럼 형식

**선택**: 호스트명만(포트 제외), **대소문자 원본 유지**, `www.` prefix **유지**.

**근거**:
- host별 token bucket 키가 `host` 컬럼 값을 그대로 사용한다(DECISIONS §5). enqueue와 bucket lookup의 정규화 규칙이 어긋나면 버킷이 분할된다 → enqueue 시점 단일 규칙으로 고정.
- 대소문자 보존: 도메인은 대소문자 무시지만, 원본 URL에서 추출한 값 그대로 유지하는 편이 디버깅에 유리. 단, 같은 호스트가 대소문자만 다르게 들어오면 unique는 `url_hash`로 잡히고, bucket은 둘을 별개 키로 취급한다(운영상 실제 발생은 드묾).
- `www.` 유지: canonical URL 규칙이 `www.` 제거를 하지 않으므로 `host`도 일관되게 유지.

### Decision 6: `score` 타입은 `DOUBLE PRECISION` (0.0 ~ 1.0)

**선택**: `DOUBLE PRECISION` 고정.

**근거**: semanticPriorityModifier 등이 0~1 구간의 실수 가중치를 곱해 최종 score를 낸다. `INTEGER`로 두면 정밀도 손실이 누적된다. PG 8 bytes 고정이라 인덱스 성능 저하 없음. (Open Question에서 종결)

### Decision 7: `last_updated_at`은 application이 매 UPDATE 시 명시 set

**선택**: trigger 없이 application 책임.

**근거**: sqlc 기반 코드베이스 컨벤션과 일관. 디버깅 용이.

### Decision 8: `harvester_frontier`는 `depth` 컬럼을 두지 않는다

**선택**: `depth`는 `pioneer_frontier`에만 둔다.

**근거**: depth는 BFS 진행 지표로 Pioneer의 사이트 탐색 단계에서만 의미가 있다. Harvester는 개별 URL의 콘텐츠 파싱에만 관여하므로 depth를 쓰지 않는다. 후속 change에서 depth 기반 Harvester 정책이 필요해지면 그때 추가.

## Risks / Trade-offs

- **`next_*_at`의 이중 의미** → lease marker 규약을 spec에 명문화하고, 내부 문서/쿼리 주석으로 보강. claim 경로에서 `next_*_at > now()`인 row는 in-flight로 간주.
- **partial index에 `next_*_at <= now()` 미포함** → backoff 대기 중인 row가 인덱스에 포함되지만 쿼리에서 제외됨. 정렬 키에 `next_*_at ASC`를 두어 실제 처리 가능한 row부터 순회하도록 보완. 대기 row 수가 과도해지면 후속 change에서 archive 테이블로 분리.
- **status 컬럼 부재로 인한 디버깅 비용** → 운영자가 row 상태를 한눈에 보기 어려움 → SQL view (`pioneer_frontier_status_v`, `harvester_frontier_status_v`)를 후속 change에서 제공 가능.
- **MaxNodesPerSite 동작 변경 보류** → bot spec의 기존 정의 제거 직후 사이트별 노드 폭발 위험 → `scheduler-claim-api`와 짧게 묶어 진행.
- **stale edge 삭제 requirement 제거** → 후속 change에서 frontier 기반 재정의까지 stale edge가 누적될 수 있으나 harvest 정확성에는 영향 없음.
- **두 테이블 분리로 인한 조인 비용** → Pioneer 성공 → Harvester enqueue fanout은 application 레벨 두 번의 쓰기(pioneer UPDATE + harvester UPSERT)로 처리. 트랜잭션 원자성 보장은 `scheduler-claim-api`에서.

## Migration Plan

1. 본 change에서 마이그레이션 스크립트로 `pioneer_frontier`, `harvester_frontier`, `harvester_frontier_pins` 테이블과 인덱스를 추가.
2. 기존 데이터 백필은 하지 않는다(동작이 아직 frontier로 옮겨가지 않으므로).
3. 후속 change `scheduler-claim-api`에서 `URLScheduler` interface와 enqueue/claim 구현을 추가하고, Pioneer/Harvester 호출부를 교체. 이 시점에 `priority_queue.go`, `bfs_queue.go` 삭제 및 fanout 트랜잭션 경계 확정.
4. 롤백: 본 change만 단독 롤백 시 세 테이블 DROP. 사용처가 아직 없으므로 안전.

## Open Questions

- `depth`를 frontier 단계에서 Pioneer만 유지하는 것으로 확정 (Decision 8). Harvester depth는 필요 시 후속 change.
- (해결됨) ~~`score` 타입~~ → `DOUBLE PRECISION` 확정 (Decision 6).
- (해결됨) ~~`normalized_url`의 길이 제한~~ → `url_hash BYTEA` (32바이트, CHECK로 강제)로 unique 제약을 분리하여 길이 무관 (Decision 3).
