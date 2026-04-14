## Why

Pioneer 재실행 시 그래프가 증분되지 않는다. `crawl()` 루프에서 자식 링크의 `CreateNode`가 duplicate key를 반환하면 해당 링크를 큐에 넣지 않고 `continue`하기 때문이다. BFS 큐 관리가 DB 상태에 의존하고 있어, 이전 실행에서 `MaxNodesPerSite` 한도로 중단된 탐색을 이어갈 수 없다. 재실행하면 루트의 직속 자식이 전부 duplicate key → 큐에 0개 → depth 1에서 종료된다.

큐에 넣을지 여부는 BFS 탐색 로직(`visited` 맵)이 결정해야 하고, DB에 노드가 있는지 여부는 관여하면 안 된다. DB는 저장소이고, BFS는 탐색 알고리즘이다. 두 관심사가 분리되어야 한다.

## What Changes

- **BFS 큐 로직 수정**: `CreateNode`가 duplicate key를 반환해도 해당 링크를 큐에 넣도록 변경. 큐 push 여부는 오직 `visited` 맵 기준으로 결정한다.
- **기존 노드 재처리 최적화**: 이미 DB에 존재하는 노드를 큐에서 꺼내 처리할 때, HTML을 다시 fetch하되 `nodesProcessed` 카운터에는 새로 생성된 노드만 집계하여 `MaxNodesPerSite` quota가 기존 노드 재방문에 소모되지 않게 한다.
- **Stale 노드 unlink**: 크롤 완료 후, 이번 세션에서 edge가 재확인되지 않은 기존 edge를 삭제한다. 사이트 구조가 변경되어 더 이상 연결되지 않는 옛날 노드의 edge를 정리한다.

## Capabilities

### New Capabilities

_(없음)_

### Modified Capabilities

- `bot`: Pioneer의 BFS 큐 관리 로직 변경. `crawl()` 메서드에서 큐 push 결정이 DB 상태와 분리된다. 크롤 완료 후 stale edge 정리 기능 추가.

## Impact

- **파일 변경**: `apps/api/internal/bot/pioneer.go` — `crawl()` 메서드의 duplicate key 핸들링 및 stale edge cleanup 로직
- **DB 쿼리**: stale edge 삭제를 위한 sqlc 쿼리 추가 필요 (`DeleteEdgesNotIn` 또는 유사)
- **동작 변경**: Pioneer 재실행 시 기존 depth 1에서 멈추던 것이 전체 BFS 탐색을 수행하며 새 노드를 증분 추가. 사라진 링크의 edge가 정리됨.
- **하위 호환성**: 외부 API 변경 없음. 기존 데이터는 다음 크롤 시 자연스럽게 증분 모드로 전환.
