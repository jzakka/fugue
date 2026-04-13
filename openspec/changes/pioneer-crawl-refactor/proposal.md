## Why

Pioneer의 `crawl()` 메서드는 현재 링크 추출, 필터링, 중복 체크를 모두 인라인으로 처리하고 있다. regex 기반 `parseLinks()`, 수동 도메인/확장자 체크, visited 맵 조회가 하나의 거대한 for 루프 안에 섞여 있어 가독성과 테스트 가능성이 낮다. FilterChain 패턴을 도입한 세 개의 선행 변경(interface, link-extractor, filter-impl)이 완료된 후, 실제 `crawl()` 호출부를 새 구조로 교체하는 통합 작업이 필요하다.

## What Changes

- **crawl() 메서드 리팩터링**: `parseLinks()` 호출을 DOM 기반 `crawler.ExtractLinksWithSelectors()`로 교체하고, 인라인 필터 로직을 `FilterChain.Apply()`로 대체
- **FilterChain 생성**: crawl 시작 시 `NewFilterChain(DomainFilter, ExtensionFilter, PathPatternFilter, CanonicalDedupFilter)` 인스턴스화
- **방문 노드 엣지 처리**: `dedupFilter.LastVisited`에서 이미 방문한 링크의 엣지를 별도로 생성
- **우선순위 계산 개선**: `NodeTypePriority(linkType) + semanticPriorityModifier(link)`로 복합 우선순위 적용
- **classifyURL() 정리**: skip 패턴 블록 제거 (PathPatternFilter가 담당), `regexp.MustCompile()` 패키지 레벨 변수로 추출
- **parseLinks() 삭제**: regex 기반 헬퍼 함수를 제거하고 DOM 기반 추출기로 완전 대체
- **테스트 업데이트**: `TestClassifyURL`에서 skip 패턴 테스트 케이스 제거 또는 기대값 변경

## Capabilities

### New Capabilities

(없음 — 이 변경은 기존 기능의 내부 구현을 리팩터링하는 것이며 새로운 capability를 추가하지 않는다)

### Modified Capabilities

- `bot`: classifyURL의 skip 패턴 제거로 "불필요한 URL 제외" 요구사항의 구현 위치가 변경됨 (classifyURL → PathPatternFilter). URL 분류의 행동 요구사항은 동일하나, skip 타입 반환 대신 FilterChain에서 사전 필터링하는 방식으로 전환.

## Impact

- **파일 변경**: `apps/api/internal/bot/pioneer.go`, `apps/api/internal/bot/helpers.go`, `apps/api/internal/bot/pioneer_test.go`
- **새 import**: `github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler` 패키지 의존성 추가
- **의존성**: pioneer-link-filter-interface, pioneer-link-extractor-selectors, pioneer-link-filter-impl 세 변경이 모두 완료된 후에야 적용 가능
- **하위 호환성**: 외부 API/동작 변경 없음. 내부 구현만 변경. 단, `NodeTypeSkip`이 classifyURL에서 더 이상 반환되지 않으므로 이를 직접 참조하는 코드가 있다면 영향받음
