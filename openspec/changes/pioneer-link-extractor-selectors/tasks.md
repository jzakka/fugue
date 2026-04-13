## 1. 사전 확인

- [ ] 1.1 `pioneer-link-filter-interface` change의 `Link`, `Selector` 타입이 `crawler` 패키지에 정의되어 있는지 확인. 없으면 해당 change를 먼저 구현

## 2. ExtractLinksWithSelectors 구현

- [ ] 2.1 `apps/api/internal/bot/crawler/link_extractor.go`에 selector 대상 태그 판별 헬퍼 함수 추가: `isSelectorTarget(tagName string) bool` — 시맨틱 태그(`nav`, `main`, `aside`, `footer`, `header`, `article`, `section`) + structural 태그(`body`)에 대해 true 반환
- [ ] 2.2 `link_extractor.go`에 `divClassSelector(node *html.Node) (Selector, bool)` 헬퍼 추가 — div에 class 속성이 있으면 `Selector{TagName: "div", Class: classValue}` 반환, 없으면 `Selector{}, false` 반환
- [ ] 2.3 `link_extractor.go`에 exported 함수 `ExtractLinksWithSelectors(body io.Reader, baseURL string) ([]Link, error)` 구현:
  - `html.Parse(body)`로 DOM 트리 생성
  - 재귀 `visit(node, ancestors []Selector)` 함수 정의
  - ElementNode 진입 시: 시맨틱 태그이면 ancestors에 push, class 있는 div면 push, 그 외 무시
  - `<a>` 발견 시: href 추출 → `makeAbsoluteURL` → `normalizeURL` → `Link{URL, Selectors: copy(ancestors)}` 생성
  - javascript:/mailto: 및 빈 href 필터링 (기존 extractLinks와 동일)
- [ ] 2.4 기존 `extractLinks()` 함수가 변경되지 않았는지 확인 (서명, 동작 모두 그대로)
- [ ] 2.5 `go build ./apps/api/internal/bot/crawler/...` 실행하여 컴파일 에러 없음 확인

## 3. 테스트 작성

- [ ] 3.1 `apps/api/internal/bot/crawler/link_extractor_test.go` 파일 생성
- [ ] 3.2 `TestExtractLinksWithSelectors_NavLinks` — `<nav>` 내부 링크가 Selectors에 `nav` 포함하는지 검증
- [ ] 3.3 `TestExtractLinksWithSelectors_FooterLinks` — `<footer>` 내부 링크가 Selectors에 `footer` 포함하는지 검증
- [ ] 3.4 `TestExtractLinksWithSelectors_MainLinks` — `<main>` 내부 링크가 Selectors에 `main` 포함하는지 검증
- [ ] 3.5 `TestExtractLinksWithSelectors_AsideLinks` — `<aside>` 내부 링크가 Selectors에 `aside` 포함하는지 검증
- [ ] 3.6 `TestExtractLinksWithSelectors_NestedStructure` — `<body><main><div class="content"><a>` 중첩 구조에서 전체 ancestor 경로가 `[Selector{TagName: "body"}, Selector{TagName: "main"}, Selector{TagName: "div", Class: "content"}]` 순서의 Selector 구조체 배열인지 검증
- [ ] 3.7 `TestExtractLinksWithSelectors_SkipsJavascriptMailto` — javascript:/mailto: 링크 제외 검증
- [ ] 3.8 `TestExtractLinksWithSelectors_SkipsEmptyHref` — 빈 href 제외 검증
- [ ] 3.9 `TestExtractLinksWithSelectors_ClasslessDivIgnored` — class 없는 div가 Selectors에 포함되지 않는지 검증

## 4. 검증

- [ ] 4.1 `go test ./apps/api/internal/bot/crawler/...` 전체 테스트 통과 확인
- [ ] 4.2 `go vet ./apps/api/internal/bot/crawler/...` 정적 분석 통과 확인
- [ ] 4.3 기존 `bfs_crawler_test.go` 테스트가 여전히 통과하는지 확인 (기존 extractLinks 무변경 검증)
