## Why

Harvester의 BFS 순회 로직이 누락되어 있어 Pioneer가 구축한 그래프를 활용할 수 없다. 현재 Harvester는 모든 노드를 단순히 우선순위로 정렬해서 순회하지만, 그래프의 엣지 관계를 따라 BFS로 탐색해야 링크 연결성을 고려한 효율적인 크롤링이 가능하다.

## What Changes

- Harvester에 BFS 기반 그래프 순회 로직 추가
  - `bot_graph_edges` 테이블을 참조하여 연결된 노드를 탐색
  - 방문한 노드를 추적하여 중복 방문 방지
  - 깊이 제한(depth limit) 적용 가능
- 노드 타입 우선순위를 BFS 큐 내에서 적용
  - 같은 깊이 레벨 내에서 listing > gallery > category > detail 순으로 처리
- 시작 노드(seed) 선택 로직 구현
  - 루트 URL에 해당하는 노드를 시작점으로 사용

## Capabilities

### New Capabilities

### Modified Capabilities
- `bot-harvester-crawler`: 그래프 순회 방식을 단순 정렬에서 BFS 기반 엣지 순회로 변경하고, 레벨별 우선순위 적용 및 순환 참조 방지를 추가한다

## Impact

- `apps/api/internal/bot/harvester.go`: BFS 순회 로직 추가, 기존 `sortNodesByPriority` 방식 대체
- `apps/api/internal/bot/repository.go`: 노드의 outgoing edges 조회 메서드 추가 필요 (GraphRepository 인터페이스)
- `apps/api/internal/bot/harvester_test.go`: BFS 로직 검증을 위한 테스트 케이스 추가
