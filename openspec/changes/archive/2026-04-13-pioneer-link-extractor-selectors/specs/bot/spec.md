## ADDED Requirements

### Requirement: DOM ancestor selector를 포함하여 링크를 추출한다
시스템은 HTML에서 링크를 추출할 때 각 링크의 DOM ancestor selector 경로를 함께 반환해야 한다(SHALL). 기존 `extractLinks()` 함수는 변경하지 않아야 한다(SHALL).

#### Scenario: 시맨틱 태그 내 링크의 selector 추적
- **WHEN** `<nav>` 내부에 `<a href="https://example.com/page">` 가 있을 때
- **THEN** 해당 Link의 Selectors에 `nav` 태그가 포함된다

#### Scenario: footer 내부 링크의 selector 추적
- **WHEN** `<footer>` 내부에 `<a href="https://example.com/about">` 가 있을 때
- **THEN** 해당 Link의 Selectors에 `footer` 태그가 포함된다

#### Scenario: main 영역 링크의 selector 추적
- **WHEN** `<main>` 내부에 `<a href="https://example.com/article">` 가 있을 때
- **THEN** 해당 Link의 Selectors에 `main` 태그가 포함된다

#### Scenario: aside 영역 링크의 selector 추적
- **WHEN** `<aside>` 내부에 `<a href="https://example.com/sidebar">` 가 있을 때
- **THEN** 해당 Link의 Selectors에 `aside` 태그가 포함된다

#### Scenario: 중첩 구조에서 전체 ancestor 경로 기록
- **WHEN** `<body><main><div class="content"><a href="https://example.com/deep">` 구조에서 링크를 추출할 때
- **THEN** 해당 Link의 Selectors에 TagName이 'body'인 Selector, TagName이 'main'인 Selector, TagName이 'div'이고 Class가 'content'인 Selector가 순서대로 포함된다

#### Scenario: 동일 URL 추출 로직 유지
- **WHEN** `ExtractLinksWithSelectors`로 HTML을 파싱할 때
- **THEN** URL 추출, 절대 URL 변환, 정규화 로직은 기존 `extractLinks()`와 동일하게 동작한다

#### Scenario: javascript/mailto 링크 제외
- **WHEN** `<a href="javascript:void(0)">` 또는 `<a href="mailto:test@test.com">` 을 만났을 때
- **THEN** 해당 링크는 결과에 포함되지 않는다

#### Scenario: 빈 href 제외
- **WHEN** `<a href="">` 또는 `<a>` (href 없음) 을 만났을 때
- **THEN** 해당 링크는 결과에 포함되지 않는다

---

### Requirement: Selector 기록은 시맨틱 및 구조적 태그로 제한한다
시스템은 DOM ancestor를 추적할 때 의미 있는 태그만 Selector로 기록해야 한다(SHALL).

#### Scenario: 시맨틱 HTML5 태그 기록
- **WHEN** ancestor에 `nav`, `main`, `aside`, `footer`, `header`, `article`, `section` 태그가 있을 때
- **THEN** 해당 태그명이 Selector로 기록된다

#### Scenario: class가 있는 div 태그 기록
- **WHEN** ancestor에 `<div class="sidebar-widget">` 이 있을 때
- **THEN** TagName이 'div'이고 Class가 'sidebar-widget'인 Selector가 기록된다

#### Scenario: class가 없는 div 태그 무시
- **WHEN** ancestor에 class 속성이 없는 `<div>` 가 있을 때
- **THEN** 해당 div는 Selector에 기록되지 않는다

#### Scenario: body 태그 기록
- **WHEN** `<body>` 태그가 ancestor에 있을 때
- **THEN** `body`가 Selector로 기록된다

#### Scenario: span, p 등 인라인/블록 태그 무시
- **WHEN** ancestor에 `<span>`, `<p>`, `<ul>`, `<li>` 등 비시맨틱 태그가 있을 때
- **THEN** 해당 태그는 Selector에 기록되지 않는다

---

### Requirement: 기존 extractLinks 함수는 변경하지 않는다
기존 unexported `extractLinks()` 함수는 현재 서명과 동작을 유지해야 한다(SHALL).

#### Scenario: BFSCrawler의 링크 추출 동작 유지
- **WHEN** `BFSCrawler`가 `extractLinks(body, baseURL)`를 호출할 때
- **THEN** 기존과 동일하게 `[]string` 타입의 URL 배열을 반환한다

#### Scenario: 기존 함수 서명 보존
- **WHEN** `extractLinks` 함수의 서명을 확인할 때
- **THEN** `func extractLinks(body io.Reader, baseURL string) ([]string, error)` 서명이 유지된다
