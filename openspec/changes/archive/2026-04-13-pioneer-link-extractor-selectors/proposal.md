## Why

Pioneer 크롤러의 Link Filter Chain에서 링크의 DOM 위치 정보가 필요하다. 현재 `extractLinks()`는 URL 문자열만 반환하므로, 링크가 `<nav>`, `<footer>`, `<aside>`, `<main>` 등 어떤 시맨틱 영역에 위치하는지 알 수 없다. DOM ancestor selector 정보가 있으면 네비게이션 링크, 푸터 링크, 본문 링크를 구분하여 필터링 우선순위를 다르게 적용할 수 있다.

## What Changes

- `link_extractor.go`에 새 exported 함수 `ExtractLinksWithSelectors(body io.Reader, baseURL string) ([]Link, error)` 추가
  - 기존 `extractLinks()` 와 동일한 URL 추출 로직 유지
  - DOM 트리 순회 시 ancestor element를 `[]Selector` 스택으로 추적
  - 각 `<a>` 요소에 대해 전체 ancestor selector 경로를 기록
  - 반환 타입을 `[]Link` 구조체로 변경 (URL + Selectors)
- 기존 unexported `extractLinks()` 함수는 변경 없음 (BFSCrawler에서 계속 사용)
- 새 테스트 파일 `link_extractor_test.go` 추가
  - `<nav>`, `<footer>`, `<main>`, `<aside>` 내부 링크의 selector 배열 검증
  - 중첩 구조 `<body><main><div class="content"><a>` 의 전체 경로 검증

## Capabilities

### New Capabilities

없음. 기존 `bot` capability 내 link extractor 확장.

### Modified Capabilities

- `bot`: Pioneer의 링크 추출 기능에 DOM selector 추적 기능 추가. `extractLinks()`는 유지하면서 새 `ExtractLinksWithSelectors()` 함수가 `Link` 구조체 배열을 반환하도록 확장.

## Impact

- 변경 파일: `apps/api/internal/bot/crawler/link_extractor.go` (함수 추가)
- 신규 파일: `apps/api/internal/bot/crawler/link_extractor_test.go` (테스트)
- 의존성: `pioneer-link-filter-interface` change에서 정의하는 `Link`, `Selector` 타입 필요
- 기존 유틸: `makeAbsoluteURL()`, `normalizeURL()` (url_utils.go) 재사용
- 기존 `BFSCrawler`는 여전히 `extractLinks()`를 사용하므로 영향 없음
