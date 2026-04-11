## Context

Harvester는 Pioneer가 구축한 사이트 그래프를 순회하며 스크립트를 실행하여 콘텐츠를 추출한다. 현재 구현은 모든 노드를 가져와 타입별 우선순위로 정렬하여 순차 처리하지만, `bot_graph_edges` 테이블에 저장된 링크 연결 정보를 활용하지 못한다.

Pioneer는 BFS로 사이트를 탐색하며 페이지 간 링크 관계를 엣지로 저장한다. Harvester도 같은 순서로 탐색해야 다음 이유로 효율적이다:
- **링크 구조 활용**: listing → gallery → detail 순의 자연스러운 흐름
- **조기 실패 탐지**: 상위 노드가 실패하면 하위 노드 처리를 건너뛸 수 있음
- **캐시 효율**: 연결된 페이지를 연속 처리하여 HTTP 캐싱 활용

제약사항:
- GraphRepository 인터페이스는 기존 메서드만 제공 (outgoing edges 조회 미지원)
- 노드 타입 우선순위는 유지해야 함 (같은 깊이 내에서 listing > detail)
- 기존 rate limiting, retry 로직과 호환되어야 함

## Goals / Non-Goals

**Goals:**
- BFS 알고리즘으로 그래프 노드를 순회하는 로직 구현
- 엣지를 따라 연결된 노드를 탐색하며 방문 추적
- 같은 깊이 레벨 내에서 노드 타입 우선순위 적용
- 시작 노드(루트 또는 listing) 자동 선택

**Non-Goals:**
- Pioneer의 BFS 로직 변경 (Pioneer는 기존 방식 유지)
- 순환 참조 감지 또는 최단 경로 탐색 (단순 BFS 순회만)
- 동적 우선순위 조정 (노드 타입 우선순위는 고정)

## Decisions

### Decision 1: 기존 GetEdgesByNode 메서드 활용
**선택**: GraphRepository의 기존 `GetEdgesByNode(ctx, fromNodeID)` 메서드를 사용하여 outgoing edges 조회
**이유**:
- 이미 구현되어 있어 추가 작업 불필요
- `[]db.BotGraphEdge` 반환값에서 `to_node_id` 추출하면 됨
- SQL 쿼리: `SELECT * FROM bot_graph_edges WHERE from_node_id = $1`

구현 예시:
```go
edges, err := h.graphRepo.GetEdgesByNode(ctx, nodeID)
if err != nil {
    return nil, err
}
childIDs := make([]uuid.UUID, len(edges))
for i, e := range edges {
    childIDs[i] = e.ToNodeID
}
```

**대안 고려**:
- 새로운 GetNodeEdges 메서드 추가: 불필요한 중복

### Decision 2: 우선순위 큐를 깊이별로 분리
**선택**: 각 BFS 레벨(깊이)에서 노드를 우선순위 큐에 넣고 정렬 후 처리
**이유**:
- BFS는 레벨별로 처리해야 깊이 보장
- 같은 레벨 내에서만 타입 우선순위 적용
- 기존 `priority_queue.go` 재사용 가능

구현:
```go
for depth := 0; !queue.IsEmpty(); depth++ {
    levelNodes := queue.PopLevel()
    sortByTypePriority(levelNodes)
    for _, node := range levelNodes {
        // process node
        // get edges and enqueue children
    }
}
```

**대안 고려**:
- 전역 우선순위 큐: BFS 깊이 순서가 깨짐
- 타입별로 큐 분리: 코드 복잡도 증가, 엣지 순서 유실

### Decision 3: 시작 노드는 루트 URL 노드 사용
**선택**: `bot_graph_nodes` 테이블에서 `url = site.root_url`인 노드를 시드로 사용
**이유**:
- Pioneer는 루트 URL에서 시작하므로 그래프에 항상 존재
- 루트에서 시작해야 전체 그래프를 커버 가능
- 명확한 시작점 정의

**대안 고려**:
- 모든 listing 노드를 시드로: 루트에서 도달 불가능한 노드가 누락될 수 있음
- depth=0인 모든 노드: Pioneer가 depth를 저장하지 않음

### Decision 4: 방문 추적은 메모리 Set 사용
**선택**: `map[uuid.UUID]bool` 사용하여 방문한 노드 추적
**이유**:
- 순환 참조 방지 (A→B→A)
- in-memory 조회가 빠름 (O(1))
- 한 사이트의 노드는 수백~수천 개로 메모리 부담 적음

**대안 고려**:
- DB에 visited 플래그 저장: 매번 UPDATE 쿼리로 성능 저하
- 방문 추적 없음: 순환 그래프에서 무한 루프 위험

## Risks / Trade-offs

**[Risk]** 순환 참조로 무한 루프 발생
→ **Mitigation**: 방문한 노드를 Set에 저장하여 재방문 방지

**[Risk]** 대규모 그래프에서 메모리 부족
→ **Mitigation**: 현실적으로 사이트당 노드는 수백~수천 개. 메모리 사용량은 무시 가능 (UUID 1개 = 16바이트, 10,000노드 = 160KB)

**[Risk]** 시작 노드가 없는 경우 (루트 URL 노드 미존재)
→ **Mitigation**: 에러 반환하고 Pioneer 재실행 안내

**[Trade-off]** BFS는 깊이 우선 탐색(DFS)보다 메모리 사용이 많음
→ 하지만 BFS가 링크 구조의 자연스러운 흐름이므로 선택. Pioneer와 일관성 유지.

**[Trade-off]** 엣지 조회를 위한 추가 DB 쿼리
→ 노드당 1회 엣지 조회 쿼리. 인덱스(`idx_graph_edges_from`)가 있어 성능 영향 미미.
