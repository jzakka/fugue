## Context

Pioneer의 `crawl()` 메서드는 BFS로 사이트를 탐색하며 노드와 엣지를 DB에 기록한다. 현재 `visited` 맵은 매 실행마다 빈 맵으로 초기화되고, 자식 링크의 `CreateNode`가 duplicate key를 반환하면 해당 링크를 큐에 넣지 않는다. 이 때문에 재실행 시 루트의 자식이 전부 duplicate → 큐에 0개 → depth 1에서 종료된다.

큐 push 여부는 BFS 탐색 로직(`visited` 맵)이 결정해야 하며, DB 노드 존재 여부와 무관해야 한다.

## Goals / Non-Goals

**Goals:**
- Pioneer 재실행 시 기존 그래프를 증분 확장 (새 노드/엣지 추가)
- `MaxNodesPerSite` quota를 새로 생성된 노드 기준으로 집계하여, 기존 노드 재방문에 quota를 소모하지 않음
- 크롤 완료 후 이번 세션에서 재확인되지 않은 edge를 삭제하여 사이트 구조 변경 반영

**Non-Goals:**
- DB에서 `visited` 맵을 미리 로드하는 방식 (오히려 BFS 탐색을 막음)
- 기존 노드의 HTML을 재활용하여 fetch를 건너뛰는 최적화 (사이트가 변경되었을 수 있으므로 항상 fetch)
- orphan 노드(edge가 하나도 없는 노드) 삭제 — 이번 범위에서 제외

## Decisions

### 1. duplicate key 시 큐에 push

**결정**: `CreateNode`가 duplicate key를 반환해도 `queue.Push()`를 수행한다.

**근거**: 큐 관리는 `visited` 맵의 책임이다. `visited`에 없으면 이번 세션에서 아직 안 본 링크이므로 큐에 넣는다. DB에 이미 있는 건 저장소의 관심사이지 탐색 알고리즘의 관심사가 아니다.

**대안 검토**: DB에서 기존 노드를 `visited`에 미리 로드하는 방안 → 로드하면 모든 기존 링크가 `visited`에 있어서 큐에 안 들어가므로 증분 탐색 불가. 기각.

### 2. `nodesProcessed` 카운터를 새 노드 기준으로 변경

**결정**: `nodesProcessed++`를 `CreateNode` 성공(새 노드 생성) 시에만 수행한다. 기존 노드 재방문은 카운트하지 않는다.

**근거**: `MaxNodesPerSite=100`은 "이 사이트에서 새로 발견할 노드 수"의 의미여야 한다. 기존 노드 300개를 재방문하면서 quota 100을 다 쓰면 새 노드를 하나도 못 만든다.

**대안 검토**: 총 처리 노드 수로 카운트하되 한도를 높이는 방안 → 사이트 크기에 따라 적절한 값을 예측할 수 없음. 기각.

### 3. Stale edge 삭제: 세션 기반 edge 추적

**결정**: 크롤 시작 시 해당 사이트의 기존 edge ID를 조회하여 `existingEdges` 집합에 저장한다. 크롤 중 edge가 재확인(CreateEdge 성공 또는 duplicate)되면 해당 ID를 `confirmedEdges` 집합에 추가한다. 크롤 완료 후 `existingEdges - confirmedEdges`에 해당하는 edge를 삭제한다.

**근거**: 사이트 구조가 변경되어 더 이상 존재하지 않는 링크의 edge를 정리해야 그래프가 현재 사이트 구조를 정확히 반영한다. edge 삭제 시 노드는 유지한다 (다른 edge로 연결될 수 있음).

**대안 검토**: (a) `last_seen_at` 타임스탬프를 edge에 추가하여 오래된 edge를 삭제 → 스키마 변경 필요, 과도. 기각. (b) 전체 edge를 삭제 후 재생성 → 크롤이 `MaxNodesPerSite` 한도로 중단되면 미방문 노드의 정상 edge도 삭제됨. 기각.

**주의**: stale edge 삭제는 BFS가 `MaxNodesPerSite` 한도에 도달해서 중단된 경우, 미방문 노드의 edge가 삭제되면 안 된다. 따라서 삭제 대상은 "이번 세션에서 방문한 노드의 outgoing edge 중 재확인되지 않은 것"으로 한정한다.

## Risks / Trade-offs

**[기존 노드 재방문 시 네트워크 비용]** → 증분 크롤의 필수 비용. 사이트 변경 감지를 위해 HTML을 다시 가져와야 하므로 fetch를 건너뛸 수 없다. rate limiting이 이미 적용되어 있어 서버 부담은 제한적.

**[대규모 사이트에서 재방문 시간 증가]** → 기존 노드 수백 개를 재방문하면 rate limiting 때문에 시간이 늘어남. 하지만 `MaxNodesPerSite`는 새 노드 기준이므로, 기존 노드 재방문이 끝나면 새 노드 탐색은 정상 진행.

**[BFS가 MaxNodes 한도에서 중단 시 stale edge 오삭제]** → 미방문 노드의 edge를 삭제하지 않도록, 삭제 범위를 "방문한 노드의 outgoing edge"로 한정. 이 조건이 정확히 구현되지 않으면 정상 edge가 삭제될 수 있음.
