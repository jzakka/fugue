## REMOVED Requirements

### Requirement: Pioneer가 DB 기존 노드와 무관하게 BFS 큐를 관리한다
**Reason**: Pioneer가 복수 프로세스로 실행될 때 인메모리 `visited` 맵으로는 중복 fetch를 막을 수 없다. URL 큐와 visited 상태 모두 영속 frontier(`bot_frontier`)로 옮겨가므로, "DB 기존 노드와 무관하게 인메모리 visited로 큐를 관리한다"는 본 requirement는 더 이상 유효하지 않다.

**Migration**: 후속 change `scheduler-claim-api`에서 frontier 기반 enqueue/claim 동작을 새 requirement로 정의한다. 동일 사이트 재실행 시의 depth 1 이상 탐색 보장은 frontier의 `last_fetched_at IS NULL` 조건으로 자연 충족된다.

---

### Requirement: MaxNodesPerSite는 새로 생성된 노드만 집계한다
**Reason**: 카운터가 단일 프로세스의 인메모리 변수(`nodesProcessed`)에 의존하던 정의이다. 복수 워커가 동일 사이트를 처리할 때 인메모리 카운터를 공유할 수 없으므로, quota 동작은 frontier 기반 집계(예: `count(*) WHERE host = ? AND last_fetched_at IS NOT NULL`)로 재정의해야 한다.

**Migration**: 후속 change `scheduler-claim-api`에서 host 단위 quota를 frontier 쿼리로 표현하는 requirement를 새로 추가한다. 그 사이 운영상의 사이트별 노드 폭발 위험은 동일 change와 짧게 묶어 진행하여 최소화한다.

---

### Requirement: 크롤 완료 후 stale edge를 삭제한다
**Reason**: "이번 세션에서 방문한 노드"라는 개념이 인메모리 BFS 세션을 전제로 한다. 복수 워커/장기 실행 모델에서는 "세션" 경계가 모호하며, stale edge 정리 로직은 frontier 기반 재방문 정책 위에서 다시 설계되어야 한다.

**Migration**: 후속 change(`scheduler-claim-api` 또는 별도의 graph-maintenance change)에서 frontier의 `last_fetched_at` 갱신 시점을 기준으로 stale edge 정리를 재정의한다. 본 change 적용 직후에는 stale edge가 일시적으로 누적될 수 있으나 harvest 정확성에는 영향이 없다.
