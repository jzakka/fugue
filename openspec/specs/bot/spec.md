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

---

### Requirement: DomainFilter가 루트 도메인 링크만 통과시킨다
DomainFilter는 LinkFilter 인터페이스를 구현하며(SHALL), 루트 도메인과 일치 여부를 판단하여 루트 도메인과 일치하는 링크만 반환해야 한다(SHALL). www 접두어 정규화를 지원해야 한다(SHALL).

#### Scenario: 동일 도메인 링크 통과
- **WHEN** DomainFilter에 루트 도메인 "example.com"이 설정되고, "https://example.com/page" 링크가 입력될 때
- **THEN** 해당 링크는 필터를 통과한다

#### Scenario: 외부 도메인 링크 차단
- **WHEN** DomainFilter에 루트 도메인 "example.com"이 설정되고, "https://other.com/page" 링크가 입력될 때
- **THEN** 해당 링크는 필터에서 제거된다

#### Scenario: www 접두어 정규화
- **WHEN** DomainFilter에 루트 도메인 "example.com"이 설정되고, "https://www.example.com/page" 링크가 입력될 때
- **THEN** 기존 도메인 비교 로직이 www 접두어를 정규화하므로 링크가 통과한다

---

### Requirement: ExtensionFilter가 미디어/정적자산 URL을 제거한다
ExtensionFilter는 LinkFilter 인터페이스를 구현하며(SHALL), 미디어, 문서, 정적자산 확장자를 판별하여 해당 확장자를 가진 URL을 필터링해야 한다(SHALL).

#### Scenario: 이미지 확장자 제거
- **WHEN** "https://example.com/photo.jpg" 또는 ".css" 확장자 URL이 입력될 때
- **THEN** 해당 링크는 필터에서 제거된다

#### Scenario: HTML 경로 통과
- **WHEN** "https://example.com/gallery/artwork-123" 처럼 제외 확장자가 없는 URL이 입력될 때
- **THEN** 해당 링크는 필터를 통과한다

---

### Requirement: PathPatternFilter가 불필요한 경로 패턴을 제거한다
PathPatternFilter는 LinkFilter 인터페이스를 구현하며(SHALL), 설정 가능한 제외 패턴 목록과 경계 인식 경로 세그먼트 매칭으로 URL을 필터링해야 한다(SHALL). 기본 제외 패턴은 "ad", "popup", "login", "signup", "cart", "checkout"이어야 한다(SHALL).

#### Scenario: 제외 패턴 경로 차단
- **WHEN** URL 경로에 "/ad/", "/popup/", "/login/" 등 제외 패턴이 세그먼트로 포함될 때
- **THEN** 해당 링크는 필터에서 제거된다

#### Scenario: 정상 경로 통과
- **WHEN** URL 경로에 제외 패턴이 포함되지 않을 때 (예: "/gallery/popular")
- **THEN** 해당 링크는 필터를 통과한다

#### Scenario: 부분 매칭 방지
- **WHEN** URL 경로에 "loading" 처럼 "login"을 부분 문자열로 포함하지만 세그먼트로는 일치하지 않을 때
- **THEN** 해당 링크는 필터를 통과한다 (경계 인식 매칭)

---

### Requirement: CanonicalDedupFilter가 정규화된 URL로 중복을 제거한다
CanonicalDedupFilter는 LinkFilter 인터페이스를 구현하며(SHALL), URL을 정규화한 후 해시 기반으로 중복을 제거해야 한다(SHALL). 이미 방문된 URL은 `LastVisited` 필드에 기록하여 크롤 루프의 엣지 생성에 활용되어야 한다(SHALL).

#### Scenario: 동일 URL 중복 제거
- **WHEN** 같은 URL이 링크 목록에 두 번 이상 포함될 때
- **THEN** 첫 번째만 통과하고 나머지는 제거된다

#### Scenario: 트래킹 파라미터 정규화 후 중복 제거
- **WHEN** "https://example.com/page?utm_source=twitter"와 "https://example.com/page"가 모두 입력될 때
- **THEN** 정규화 후 동일한 canonical URL로 인식되어 하나만 통과한다

#### Scenario: 이미 방문된 URL의 LastVisited 기록
- **WHEN** visited 맵에 이미 존재하는 URL이 입력될 때
- **THEN** 해당 URL은 필터 결과에서 제거되고, 이미 방문된 링크 정보가 외부에 보고된다

---

### Requirement: canonicalURL이 URL을 정규화한다
`canonicalURL()` 함수는 URL에서 트래킹 파라미터(utm_source, utm_medium, utm_campaign, utm_term, utm_content, ref, fbclid, gclid)를 제거하고(SHALL), www 접두어를 제거하며(SHALL), trailing slash를 통일해야 한다(SHALL).

#### Scenario: 트래킹 파라미터 제거
- **WHEN** "https://example.com/page?utm_source=twitter&id=123" URL이 입력될 때
- **THEN** "https://example.com/page?id=123"으로 정규화된다 (utm_source 제거, id 보존)

#### Scenario: www 접두어 제거
- **WHEN** "https://www.example.com/page" URL이 입력될 때
- **THEN** "https://example.com/page"로 정규화된다

#### Scenario: trailing slash 통일
- **WHEN** "https://example.com/page/" URL이 입력될 때
- **THEN** trailing slash가 제거되어 "https://example.com/page"로 정규화된다

---

### Requirement: semanticPriorityModifier가 HTML 위치 기반 우선순위를 보정한다
`semanticPriorityModifier()` 함수는 링크의 HTML 위치(Selector)에 따라 우선순위 보정값을 반환해야 한다(SHALL). (이 함수는 LinkFilter 인터페이스를 구현하지 않는 독립 헬퍼이며, `crawl-refactor` 변경에서 크롤 루프의 우선순위 계산에 사용된다)

#### Scenario: footer/aside 링크 우선순위 감소
- **WHEN** 링크의 Selector가 footer 또는 aside 영역을 나타낼 때
- **THEN** -50을 반환한다

#### Scenario: nav/header 링크 우선순위 소폭 감소
- **WHEN** 링크의 Selector가 nav 또는 header 영역을 나타낼 때
- **THEN** -20을 반환한다

#### Scenario: 본문 링크 우선순위 유지
- **WHEN** 링크의 Selector가 main, article 또는 기타 영역을 나타낼 때
- **THEN** 0을 반환한다

---

### Requirement: 필터 체인이 순서대로 필터를 적용한다
여러 LinkFilter를 순서대로 체이닝하여 적용할 수 있어야 하며(SHALL), 각 필터의 출력이 다음 필터의 입력이 되어야 한다(SHALL).

#### Scenario: 체이닝 순서 보장
- **WHEN** DomainFilter -> ExtensionFilter -> PathPatternFilter -> CanonicalDedupFilter 순서로 체인이 구성될 때
- **THEN** 링크 목록이 순서대로 각 필터를 통과하며 최종 결과가 반환된다

#### Scenario: 빈 링크 목록 처리
- **WHEN** 빈 링크 목록이 필터 체인에 입력될 때
- **THEN** 에러 없이 빈 목록이 반환된다
