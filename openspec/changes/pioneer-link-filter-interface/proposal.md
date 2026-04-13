## Why

Pioneer 크롤러의 `crawl()` 메서드에서 링크 필터링 로직이 인라인으로 하드코딩되어 있다. 도메인 검증, 파일 확장자 필터링, URL 분류 등이 모두 BFS 루프 안에 직접 박혀 있어 새로운 필터(예: CSS selector 기반 필터, robots.txt 준수 필터) 추가 시 `pioneer.go`를 매번 수정해야 한다. 이를 인터페이스와 체인 패턴으로 분리하여 확장성과 테스트 용이성을 확보한다.

## What Changes

- `crawler/link.go` 신규: DOM 구조 정보를 담는 `Selector`/`Link` 타입 정의. 현재 `extractLinks`가 `[]string`만 반환하는데, 향후 CSS selector 기반 필터링을 위해 DOM 경로 정보를 함께 전달하는 구조체가 필요하다.
- `bot/link_filter.go` 신규: `LinkFilter` 인터페이스와 `FilterChain` 구조체 정의. 필터를 순차적으로 적용하는 Chain of Responsibility 패턴의 기반.
- 이 변경은 **인터페이스/타입 정의만** 포함하며, 구현체는 포함하지 않는다.

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities

- `bot`: Pioneer의 링크 필터링 프로세스에 인터페이스 기반 필터 체인 구조를 도입. 기존 인라인 필터링 로직을 표준 인터페이스로 추상화하며, DOM 구조 정보를 포함하는 Link 타입을 정의한다.

## Impact

- **코드**: `apps/api/internal/bot/crawler/link.go` (신규), `apps/api/internal/bot/link_filter.go` (신규)
- **의존성**: 외부 의존성 없음. 순수 Go 타입/인터페이스 정의.
- **하위 변경**: `filter-impl` (구체 필터 구현), `link-extractor` (extractLinks 반환 타입 변경), `crawl-refactor` (Pioneer.crawl에서 FilterChain 사용)가 이 변경에 의존한다.
