## Why

Pioneer/Harvester는 단일 프로세스에서만 동작 가능한 인메모리 BFS 큐(`PriorityQueue`, `BFSQueue`)에 의존하고 있어, 복수 워커로 수평 확장하면 동일 URL을 중복 fetch하거나 stale edge 정리가 깨진다. 또한 한 사이트의 BFS 진행 상태가 프로세스 재시작 시 모두 휘발되어 재실행 시 루트부터 다시 탐색해야 하므로, 운영적으로 신뢰할 수 있는 큐가 필요하다. URLScheduler를 Postgres 기반 frontier 테이블로 교체하기 위한 첫 단계로, 우선 **테이블 스키마**를 확정한다.

추가로, Pioneer와 Harvester는 동작 역할이 근본적으로 다르다. Pioneer는 한 URL을 fetch하면 결과가 (a) 새로 발견된 링크들(다시 Pioneer 큐로) + (b) 원본 URL의 snapshot(Harvester 큐로)로 **fanout**된다. 두 경로의 claim 조건·에러 카운터·재크롤 정책이 서로 다르므로, 단일 테이블 + `queue_type` 컬럼으로 묶으면 partial index가 두 배로 불어나고 재크롤/재harvest 정책이 서로 뒤엉킨다. 따라서 Pioneer 큐와 Harvester 큐를 **독립된 두 테이블**로 분리하고, Pin과의 1:N 관계는 별도 조인 테이블로 표현한다.

## What Changes

- 새 테이블 3종을 정의:
  - `pioneer_frontier`: Pioneer가 fetch 대상 URL을 담는 큐. fetch 상태·실패 카운터·재fetch 스케줄을 보관.
  - `harvester_frontier`: Pioneer가 fetch에 성공한 URL을 Harvester 소비용으로 fanout해 쌓는 큐. harvest 상태·실패 카운터·snapshot 참조를 보관.
  - `harvester_frontier_pins`: `harvester_frontier` 1 row ↔ 여러 Pin의 조인 테이블 (ScriptAdapter가 N개 Pin을 생성할 수 있으므로 1:N).
- 두 frontier 테이블은 각각 `url_hash BYTEA` (sha256 raw, 32 bytes — 길이는 `CHECK (octet_length(url_hash) = 32)` 제약으로 강제, 값은 `sha256(normalized_url)`)에 unique constraint를 두어 중복 enqueue를 DB 레벨에서 방지.
- **`status` enum 컬럼은 두지 않는다.** 시간/카운터 컬럼 조합으로 상태를 표현한다.
- **Partial index** (각각 부분 조건과 정렬 키를 분리 표기):
  - `pioneer_frontier`:
    - partial condition: `fetch_error_count < 5`
    - sort keys: `score DESC, next_fetch_at ASC`
  - `harvester_frontier`:
    - partial condition: `harvested_at IS NULL AND harvest_error_count < 5`
    - sort keys: `score DESC, next_harvest_at ASC`
- `next_fetch_at` / `next_harvest_at`은 **in-flight marker 겸용**. claim 시 lease timeout(10분)만큼 미래로 갱신하여 두 워커가 동시에 같은 row를 잡지 않도록 한다.
- **BREAKING**: bot spec에서 인메모리 BFS 전제 requirement 3건 제거 (Pioneer BFS 큐, MaxNodesPerSite 새 노드만 집계, stale edge 삭제). 본 change의 frontier 테이블이 이를 대체하며, 후속 change에서 세부 동작을 frontier 기반으로 재정의한다.
- `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue`는 후속 change(`scheduler-claim-api`)에서 `URLScheduler`로 rename하여 본 테이블들을 백엔드로 사용.

## Capabilities

### New Capabilities
- `scheduler`: Pioneer/Harvester가 각자 사용하는 영속적 URL frontier. 본 change에서는 테이블 스키마(두 frontier 테이블 + 조인 테이블), 인덱스, 컬럼 의미, lease marker 규약만 정의한다. claim 쿼리, backoff 산식, host token bucket 등 동작 규칙은 후속 change(`scheduler-claim-api`, `scheduler-retry-backoff`, `scheduler-host-token-bucket`)에서 추가한다.
- **스펙 범위 명시**: 본 change의 `scheduler` spec은 "스키마 계약(schema-as-contract)"으로, 컬럼명·타입·SQL 문법 등 구현 결합 수준의 요구사항을 포함한다. 이는 후속 change들이 동일 테이블 위에 기능을 쌓기 위한 안정된 인터페이스를 제공하기 위함이다. 후속 change(`scheduler-claim-api` 등)는 이 스키마 위에서 행위 계약(claim 쿼리, 락 전략 등)을 추가한다.
- **본 change가 포함하는 행위의 경계**: row 생성 시 초기 상태, lease marker 규약(`next_*_at = now() + 10 minutes`로 세팅한다는 "규약"), 성공/실패 시 갱신할 컬럼과 의미, UPSERT 조건 등 "스키마가 의미를 가지기 위해 필요한 최소 행위"는 본 change에 포함된다. claim 쿼리 자체(`SELECT ... FOR UPDATE SKIP LOCKED`, 트랜잭션 경계, host 단위 동시성 제어)는 `scheduler-claim-api`에서 다룬다.

### Modified Capabilities
- `bot`: 인메모리 BFS 큐를 전제로 한 requirement 3건을 제거한다. (재정의는 후속 change에서 frontier 기반으로 수행)
  - "Pioneer가 DB 기존 노드와 무관하게 BFS 큐를 관리한다"
  - "MaxNodesPerSite는 새로 생성된 노드만 집계한다"
  - "크롤 완료 후 stale edge를 삭제한다"

## Impact

- **DB 스키마**: 새 테이블 3개(`pioneer_frontier`, `harvester_frontier`, `harvester_frontier_pins`) 추가, 각 frontier 테이블에 unique 인덱스 1개 + partial index 1개. sqlc 마이그레이션 1건.
- **코드**: 본 change 범위에서는 테이블/마이그레이션만 추가. `priority_queue.go`, `bfs_queue.go`는 후속 change에서 제거 예정.
- **운영**: Pioneer/Harvester를 복수 인스턴스로 배포할 수 있는 기반이 마련된다. 단, 본 change 단독으로는 동작 변경이 없다(스키마만 도입).
- **문서**: `docs/erd.md`에 세 테이블 추가.
