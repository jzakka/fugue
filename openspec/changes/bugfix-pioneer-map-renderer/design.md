## Context

Pioneer는 BFS로 사이트를 크롤하면서 노드(페이지)를 발견하고 DB에 저장합니다. 그러나 현재 `crawl()` 루프에서 부모→자식 edge를 생성하는 코드가 누락되어 있어, 노드만 존재하고 관계가 없는 상태입니다.

D3 force-directed graph 시각화도 두 가지 결함이 있습니다:
1. `d3.forceLink`가 `source`/`target` 프로퍼티를 기대하는데, 서버에서 내려주는 edge 데이터는 `from_node_id`/`to_node_id`를 사용합니다.
2. `ticked()` 콜백에서 매 tick마다 `data.nodes.find()`로 O(n) 탐색을 하고 있어 비효율적이며, forceLink가 edge를 resolve하지 못하면 선이 그려지지 않습니다.

URL 분류(`classifyURL`)는 키워드 매칭 → 정규식 → 기본값(listing) 순서로 동작하는데, 대부분의 URL이 어떤 키워드에도 매칭되지 않아 전부 listing으로 분류됩니다.

## Goals / Non-Goals

**Goals:**
- Pioneer BFS 루프에서 edge를 생성하여 그래프 구조를 완성합니다.
- D3 렌더러가 edge 데이터를 올바르게 처리하여 노드 간 연결선을 표시합니다.
- URL 분류 로직을 개선하여 다양한 페이지 타입을 구분합니다.
- 노드 타입별 시각적 구분(색상)으로 그래프의 가독성을 높입니다.

**Non-Goals:**
- URL 분류 로직의 사이트별 커스터마이징 (향후 사이트별 규칙 시스템으로 별도 대응)
- D3 시각화의 전면 재작성 (기존 force-directed 방식 유지)
- Pioneer의 크롤링 전략 자체 변경 (BFS 순서, 우선순위 등)
- Pioneer BFS를 `bot/crawler` 패키지로 마이그레이션 (`crawler.BFSCrawler`는 depth/parentURL 추적이 있는 깔끔한 구현이지만, Pioneer의 인라인 BFS를 이 패키지로 교체하는 것은 이번 버그 수정 범위를 넘어서므로 별도 변경으로 다룸)

## Decisions

### 1. Edge 생성 위치: BFS 루프 내 링크 추가 시점

**결정**: `crawl()` 루프에서 자식 링크를 큐에 추가하기 직전, 부모 노드 ID와 자식 노드 ID로 `CreateEdge()`를 호출합니다.

**필요한 선행 리팩터링**:
- 현재 `GetNodeByHash`와 `CreateNode`의 반환값이 `_`로 버려지고 있으므로, **현재 처리 중인 노드의 ID를 변수에 캡처**해야 합니다.
- `visited` 맵을 `map[string]bool`에서 `map[string]uuid.UUID`로 변경하여, 이미 방문한 노드에 대해서도 node ID를 조회할 수 있게 합니다.

**Edge 생성 흐름**:
1. 현재 URL 처리 → `GetNodeByHash` 또는 `CreateNode`에서 `currentNodeID` 캡처
2. 링크 파싱
3. 각 링크에 대해:
   - 미방문 링크: 자식 노드 `CreateNode` → `childNodeID` 획득 → `CreateEdge(currentNodeID, childNodeID)` → 큐에 push + visited에 `hash→childNodeID` 저장
   - 이미 방문한 링크: `visited[hash]`에서 `childNodeID` 조회 → `CreateEdge(currentNodeID, childNodeID)` (큐 push 없음, 중복 edge는 DB에서 무시)

**근거**: 부모 노드는 이미 생성된 상태이고, 자식 노드는 링크 발견 시점에 생성(또는 조회)합니다. QueueItem 구조를 변경할 필요가 없으며, 현재 BFS 루프의 스코프 내에서 모든 정보가 확보됩니다.

**대안 검토**:
- QueueItem에 parentNodeID 필드 추가 후 큐에서 꺼낼 때 edge 생성 → 구조 변경이 크고, 이미 방문한 노드에 대한 edge 생성이 누락됨
- 별도 후처리 단계에서 일괄 생성 → 크롤 중 실패 시 일부 edge가 누락될 수 있음

### 2. D3 edge 매핑: 서버 측에서 변환

**결정**: Go 서버의 `SerializeGraphData`에서 edge를 JSON으로 변환할 때 `from_node_id`/`to_node_id`에 더해 `source`/`target` 필드를 추가합니다. D3 template의 `ticked()` 콜백은 forceLink가 자동 resolve한 `d.source`/`d.target` 참조를 사용하도록 수정합니다.

**근거**: D3 forceLink는 edge 객체의 `source`/`target`을 자동으로 노드 객체 참조로 치환합니다. 이를 활용하면 `ticked()`에서 매 tick O(n) 탐색이 불필요해집니다.

**대안 검토**:
- 클라이언트(JS)에서 매핑 → 가능하지만 서버에서 하는 것이 일관성이 높음
- edge JSON 필드명 자체를 `source`/`target`로 변경 → Graphviz 출력과의 일관성이 깨짐

### 3. URL 분류 개선: 쿼리 파라미터 ID 인식 추가

**결정**: `classifyURL()` 함수에 쿼리 파라미터 기반 ID 패턴과 path segment 기반 패턴을 추가합니다.

**변경 내용**:
- 경로에 `/artworks/`, `/photos/`, `/works/`, `/illust/` 등 콘텐츠 단수형 패턴 → `detail`
- 쿼리 파라미터에 `id=\d+`, `illust_id=\d+` 등 명시적 ID 파라미터 → `detail` (`?p=`는 pagination 용도가 대부분이므로 제외)
- `/contest/`, `/event/` 등 → `category`
- `/member.php`, `/users/`, `/u/` 등 프로필 패턴 → 현재는 `listing` 유지 (profile 노드 타입은 향후 추가)

### 4. 노드 타입별 색상 체계

**결정**: D3 렌더러에 타입별 색상을 적용합니다.

| 타입 | 색상 | 의미 |
|------|------|------|
| listing | `#3b82f6` (blue) | 목록/탐색 페이지 |
| gallery | `#8b5cf6` (purple) | 갤러리/컬렉션 |
| detail | `#10b981` (green) | 상세 콘텐츠 |
| category | `#f59e0b` (amber) | 카테고리/태그 |
| unknown | `#6b7280` (gray) | 미분류 |

script 구현 여부는 노드 테두리(stroke)로 표현합니다: 구현됨 = 밝은 테두리, 미구현 = 어두운 테두리.

## Risks / Trade-offs

- **[기존 데이터]** 이미 크롤된 노드에 대한 edge는 생성되지 않습니다. → **완화**: 해당 사이트를 다시 Pioneer로 크롤하면 edge가 생성됩니다. 기존 노드는 `GetNodeByHash`로 중복 방지되므로 재크롤이 안전합니다.
- **[Edge 생성 실패]** 자식 노드 생성과 edge 생성이 별도 쿼리이므로 edge 생성만 실패할 수 있습니다. → **완화**: `ON CONFLICT DO NOTHING`으로 중복 edge를 무시하고, edge 생성 실패 시 로깅만 하고 크롤을 계속합니다.
- **[URL 분류 정확도]** 키워드 기반 분류는 사이트마다 URL 규칙이 달라 100% 정확하지 않습니다. → **수용**: 현재 단계에서는 합리적인 수준의 분류면 충분합니다. 오분류는 시각화에서 색상만 잘못되는 정도의 영향입니다.
