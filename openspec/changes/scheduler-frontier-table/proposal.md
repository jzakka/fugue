## Why

Pioneer/Harvester는 단일 프로세스에서만 동작 가능한 인메모리 BFS 큐(`PriorityQueue`, `BFSQueue`)에 의존하고 있어, 복수 워커로 수평 확장하면 동일 URL을 중복 fetch하거나 stale edge 정리가 깨진다. 또한 한 사이트의 BFS 진행 상태가 프로세스 재시작 시 모두 휘발되어 재실행 시 루트부터 다시 탐색해야 하므로, 운영적으로 신뢰할 수 있는 큐가 필요하다. URLScheduler를 Postgres 기반 frontier 테이블로 교체하기 위한 첫 단계로, 우선 **테이블 스키마**를 확정한다.

## What Changes

- 새 테이블 `bot_frontier`를 정의: Pioneer fetch 상태, Harvester harvest 상태, 우선순위 점수, host 정보, 백오프용 카운터/타임스탬프 컬럼을 단일 row로 통합.
- `normalized_url`에 unique constraint를 두어 중복 enqueue를 DB 레벨에서 방지.
- **`status` enum 컬럼은 두지 않는다.** Pioneer/Harvester가 각자 필요한 조건을 쿼리 빌더로 조합하여 claim하도록 한다.
- **Pioneer claim용 partial index**: `WHERE last_fetched_at IS NULL AND fetch_error_count < 5 AND next_fetch_at <= now()` (정렬: `score DESC`).
- **Harvester claim용 partial index**: `WHERE pin_id IS NULL AND harvest_error_count < 5 AND next_harvest_at <= now()` (정렬: `score DESC`).
- `host`, `score` 보조 인덱스로 host별 token bucket 조회 및 우선순위 정렬을 지원.
- **BREAKING**: bot spec에서 인메모리 BFS 전제 requirement 3건 제거 (Pioneer BFS 큐, MaxNodesPerSite 새 노드만 집계, stale edge 삭제). 후속 change에서 frontier 기반 동작으로 재정의 예정.
- `apps/api/fuguebot_pseudo.go`의 `URLPriorityQueue`는 후속 change(`scheduler-claim-api`)에서 `URLScheduler`로 rename하여 본 테이블을 백엔드로 사용.

## Capabilities

### New Capabilities
- `scheduler`: Pioneer/Harvester가 공유하는 영속적 URL frontier. 본 change에서는 테이블 스키마, 인덱스, 컬럼 의미만 정의한다. claim API, backoff 정책, host token bucket 등 동작 규칙은 후속 change(`scheduler-claim-api`, `scheduler-retry-backoff`, `scheduler-host-token-bucket`)에서 추가한다.

### Modified Capabilities
- `bot`: 인메모리 BFS 큐를 전제로 한 requirement 3건을 제거한다. (재정의는 후속 change에서 frontier 기반으로 수행)
  - "Pioneer가 DB 기존 노드와 무관하게 BFS 큐를 관리한다"
  - "MaxNodesPerSite는 새로 생성된 노드만 집계한다"
  - "크롤 완료 후 stale edge를 삭제한다"

## Impact

- **DB 스키마**: 새 테이블 `bot_frontier` 추가, 4개 인덱스(unique, partial×2, score). sqlc 마이그레이션 1건.
- **코드**: 본 change 범위에서는 테이블/마이그레이션만 추가. `priority_queue.go`, `bfs_queue.go`는 후속 change에서 제거 예정.
- **운영**: Pioneer/Harvester를 복수 인스턴스로 배포할 수 있는 기반이 마련된다. 단, 본 change 단독으로는 동작 변경이 없다(스키마만 도입).
- **문서**: `docs/erd.md`에 `bot_frontier` 추가.
