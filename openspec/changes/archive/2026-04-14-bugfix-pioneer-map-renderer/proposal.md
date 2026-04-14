## Why

Pioneer가 BFS 크롤 중 발견한 링크 관계(부모→자식)를 DB에 기록하지 않고 있습니다. `GraphRepository.CreateEdge()`가 인터페이스에 정의되어 있고 sqlc 코드도 생성되어 있지만, `Pioneer.crawl()` 루프에서 한 번도 호출되지 않습니다. 그 결과 그래프 시각화(D3 force-directed graph)에서 노드 간 연결선이 거의 보이지 않습니다.

추가로, D3 렌더러가 edge 데이터의 `from_node_id`/`to_node_id` 필드를 `forceLink`가 기대하는 `source`/`target` 형식으로 매핑하지 않아 edge가 있어도 렌더링되지 않으며, URL 분류 로직(`classifyURL`)이 지나치게 단순하여 100개 노드 전부가 `listing`으로 분류되는 문제도 있습니다.

## What Changes

- **Pioneer edge 생성 누락 수정**: `Pioneer.crawl()` BFS 루프에서 부모 노드→자식 노드 edge를 `CreateEdge()`로 기록하도록 수정합니다.
- **D3 forceLink 호환성 수정**: edge 데이터의 `from_node_id`/`to_node_id`를 D3 `forceLink`가 인식하는 `source`/`target`으로 매핑합니다. `ticked()` 콜백의 수동 노드 조회(O(n) per edge per tick) 역시 D3가 자동 resolve한 참조를 사용하도록 수정합니다.
- **URL 분류 로직 개선**: `classifyURL()` 함수를 수정하여 쿼리 파라미터의 숫자 ID(`?id=12345`)와 다양한 사이트별 URL 패턴을 올바르게 분류합니다.
- **D3 그래프 시각적 개선**: 노드 타입별 색상 구분, edge 방향 화살표 가시성 개선, force 파라미터 튜닝으로 그래프가 사이트 구조를 의미 있게 표현하도록 합니다.

## Capabilities

### New Capabilities

_(해당 없음 — 기존 기능의 버그 수정입니다)_

### Modified Capabilities

- `bot`: Pioneer의 edge 생성 누락 수정 및 URL 분류 로직 개선 (기존 스펙의 "링크 관계 기록" 시나리오와 "URL 패턴으로 페이지 타입을 분류한다" 요구사항에 해당)

## Impact

- **Pioneer (`apps/api/internal/bot/pioneer.go`)**: `crawl()` 메서드에 edge 생성 로직 추가, `classifyURL()` 함수 개선
- **D3 시각화 템플릿 (`apps/api/cmd/bot-visualize/template.html`)**: forceLink 데이터 매핑 수정, ticked 콜백 최적화, 노드 타입별 색상 체계 적용
- **기존 테스트**: Pioneer 테스트에 edge 생성 검증 추가 필요
- **DB**: 스키마 변경 없음 (`bot_graph_edges` 테이블 이미 존재)
