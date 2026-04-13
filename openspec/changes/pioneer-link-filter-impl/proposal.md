## Why

Pioneer 크롤러의 링크 필터링 로직이 현재 `pioneer.go`에 산재해 있어, 각 필터를 독립적으로 테스트하거나 교체하기 어렵다. `LinkFilter` 인터페이스(pioneer-link-filter-interface에서 정의)를 기반으로 4개의 구체적 필터를 구현하고 체이닝할 수 있게 하면, 필터별 단위테스트가 가능해지고 도메인별 필터 구성 커스터마이징이 용이해진다.

## What Changes

- **DomainFilter 구현**: 기존 `isSameDomain()` 함수를 래핑하여 루트 도메인과 일치하는 링크만 통과시킨다.
- **ExtensionFilter 구현**: 기존 `hasExcludedExtension()` 함수를 래핑하여 미디어/문서/정적자산 확장자를 가진 URL을 제거한다.
- **PathPatternFilter 구현**: `classifyURL()`의 skip 패턴을 추출하여 `urlPathContains()` 기반 경계 인식 세그먼트 매칭으로 불필요한 경로를 필터링한다.
- **CanonicalDedupFilter 구현**: URL 정규화(tracking parameter 제거, host 정규화, trailing slash 통일) 후 해시 기반 중복 제거를 수행한다. 이미 방문한 URL은 `LastVisited`를 통해 엣지 생성에 활용된다.
- **헬퍼 함수 추가**: `canonicalPath()` (URL 정규화), `semanticPriorityModifier()` (HTML 위치 기반 우선순위 보정)
- **포괄적 단위테스트**: 각 필터별 테스트 + 체인 테스트 + 헬퍼 함수 테스트

## Capabilities

### New Capabilities
(없음)

### Modified Capabilities
- `bot`: Pioneer의 기존 인라인 필터링 함수들(`isSameDomain`, `hasExcludedExtension`, `urlPathContains`)을 `LinkFilter` 인터페이스 구현체로 래핑하고, URL 정규화 기반 중복 제거 필터와 DOM 위치 기반 우선순위 보정 헬퍼를 추가한다.

## Impact

- **코드**: `apps/api/internal/bot/link_filter.go` 신규 파일 생성, `apps/api/internal/bot/link_filter_test.go` 신규 파일 생성
- **의존성**: pioneer-link-filter-interface 변경(LinkFilter 인터페이스, Link/Selector 타입 정의)에 의존
- **기존 함수 재사용**: `isSameDomain()`, `hasExcludedExtension()`, `urlPathContains()`, `hashURL()` (pioneer.go)
- **API/DB 변경 없음**: 순수 인메모리 필터링 로직이므로 스키마나 엔드포인트 변경 없음
