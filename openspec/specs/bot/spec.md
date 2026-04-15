## Requirements

### Requirement: 그래프 노드와 엣지를 관리한다
시스템은 크롤링한 페이지를 노드로, 링크를 엣지로 저장하며 중복을 방지해야 한다. 노드는 개별 URL이 아니라 **페이지 템플릿 패턴**을 표현해야 한다(SHALL).

#### Scenario: 쿼리 파라미터가 다른 URL을 동일 노드로 합침
- **WHEN** Pioneer가 `aaa/bbb?x=1`과 `aaa/bbb?x=2`를 발견할 때
- **THEN** 시스템은 두 URL을 동일한 노드로 처리한다

#### Scenario: 숫자 ID가 다른 URL을 동일 노드로 합침
- **WHEN** Pioneer가 `/artworks/12345`와 `/artworks/67890`을 발견할 때
- **THEN** 시스템은 두 URL을 동일한 노드로 처리한다 (path 내 숫자 전용 세그먼트는 동일 패턴으로 간주)

#### Scenario: 다중 숫자 세그먼트 치환
- **WHEN** Pioneer가 `/user/123/post/456`과 `/user/789/post/012`를 발견할 때
- **THEN** 시스템은 두 URL을 동일한 노드로 처리한다

#### Scenario: 비숫자 slug는 보존
- **WHEN** Pioneer가 `/contest/magicalparty`를 발견할 때
- **THEN** 시스템은 고유한 노드로 생성한다 (숫자가 아닌 세그먼트는 구분됨)

#### Scenario: 혼합 문자열 세그먼트는 보존
- **WHEN** Pioneer가 `/item/abc123`을 발견할 때
- **THEN** 시스템은 고유한 노드로 생성한다 (순수 숫자가 아닌 세그먼트는 구분됨)

#### Scenario: 원본 URL 보존
- **WHEN** 새 패턴의 첫 번째 URL이 발견될 때
- **THEN** 시스템은 해당 원본 URL을 보존하여 이후 실제 페이지 접근에 사용할 수 있게 한다

#### Scenario: 이미 존재하는 패턴의 URL 발견
- **WHEN** 동일 패턴에 해당하는 URL이 이미 노드로 존재할 때
- **THEN** 시스템은 새 노드를 생성하지 않고 기존 노드를 재사용한다

#### Scenario: Harvester가 원본 URL로 페이지를 접근
- **WHEN** Harvester가 노드를 처리할 때
- **THEN** 시스템은 canonical path가 아닌 보존된 원본 URL을 사용하여 실제 페이지를 fetch한다

#### Scenario: 링크 관계 기록
- **WHEN** Pioneer가 BFS 크롤 중 페이지 A에서 페이지 B로의 링크를 발견할 때
- **THEN** 시스템은 B의 노드를 생성(또는 기존 노드를 조회)한 뒤 A에서 B로의 엣지를 생성한다

#### Scenario: 이미 방문한 노드로의 엣지 생성
- **WHEN** Pioneer가 페이지 A에서 이미 방문한 페이지 C로의 링크를 발견할 때
- **THEN** 시스템은 A에서 C로의 엣지를 생성한다 (중복 엣지는 무시된다)

#### Scenario: 중복 엣지 방지
- **WHEN** 같은 링크를 여러 번 발견했을 때
- **THEN** 시스템은 하나의 엣지만 유지한다

#### Scenario: 사이트의 모든 list 페이지 조회
- **WHEN** 특정 사이트의 list 타입 노드들을 조회할 때
- **THEN** 시스템은 해당하는 모든 노드를 빠르게 반환한다

---

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

---

### Requirement: 크롤된 URL 집합에서 가변 segment를 자동 탐지한다
시스템은 크롤된 URL 집합의 통계적 분석을 통해 가변 segment를 `{param}`으로 치환해야 한다(SHALL). leaf explosion과 mid-path parameterization을 모두 탐지해야 한다(SHALL). 탐지 임계값은 설정 가능해야 한다(SHALL).

#### Scenario: leaf explosion 탐지 (같은 prefix 아래 리프 폭발)
- **WHEN** `/howto/search/AIart`, `/howto/search/8bit`, `/howto/search/watercolor` 등 임계값을 초과하는 수의 URL이 같은 prefix 아래 서로 다른 마지막 segment를 가질 때
- **THEN** 시스템은 이들을 동일 패턴으로 판별하고 템플릿 `/howto/search/{param}`을 생성한다

#### Scenario: mid-path parameterization 탐지 (경로 중간의 가변 segment)
- **WHEN** `/tags/TAG1/artwork`, `/tags/TAG2/artwork`, `/tags/TAG3/artwork` 등 임계값을 초과하는 수의 URL이 중간 segment만 다르고 나머지가 동일할 때
- **THEN** 시스템은 이들을 동일 패턴으로 판별하고 템플릿 `/tags/{param}/artwork`을 생성한다

#### Scenario: 다중 suffix를 가진 mid-path 탐지
- **WHEN** `/tags/TAG1/artwork`, `/tags/TAG1/illustrations`, `/tags/TAG2/artwork`, `/tags/TAG2/illustrations` 등 각 suffix별로 임계값을 초과할 때
- **THEN** 시스템은 suffix별로 별도 패턴을 생성한다: `/tags/{param}/artwork`, `/tags/{param}/illustrations`

#### Scenario: 깊은 중첩 mid-path 탐지
- **WHEN** `/users/USER1/posts/recent`, `/users/USER2/posts/recent` 등 임계값을 초과할 때
- **THEN** 시스템은 템플릿 `/users/{param}/posts/recent`을 생성한다

#### Scenario: 정적 리소스 경로는 머지하지 않음
- **WHEN** `/api/users/{id}`, `/api/posts/{id}` 등 서로 다른 리소스 경로가 존재하고, 가변 위치 이후의 segment가 모두 이미 parameterized(`{id}` 등)일 때
- **THEN** 시스템은 이들을 별도 경로로 유지한다

#### Scenario: depth가 다른 URL은 별도 처리
- **WHEN** `/tags/photo`(depth 2)와 `/tags/photo/artwork`(depth 3)가 존재할 때
- **THEN** depth가 다르므로 서로 다른 그룹에서 독립적으로 처리된다

#### Scenario: 임계값 미달 시 머지하지 않음
- **WHEN** 같은 패턴에 해당하는 URL 수가 임계값 이하일 때
- **THEN** 시스템은 해당 URL들을 개별 노드로 유지한다

---

### Requirement: 패턴 분석 결과를 기반으로 노드를 머지한다
패턴 분석으로 동일 패턴에 속하는 DB 노드들을 하나의 대표 노드로 통합해야 한다(SHALL). 대표 노드는 가장 먼저 생성된 노드여야 한다(SHALL).

#### Scenario: 패턴 내 노드 머지
- **WHEN** `/tags/TAG1/artwork`, `/tags/TAG2/artwork`, `/tags/TAG3/artwork`가 동일 패턴으로 판별될 때
- **THEN** 가장 먼저 생성된 노드를 대표로 선택하고, 나머지 노드의 엣지를 대표 노드로 재연결한 뒤, 나머지 노드를 삭제하고, 대표 노드의 URL을 `/tags/{param}/artwork`으로 변경한다

#### Scenario: 엣지 재연결 시 중복 제거
- **WHEN** 머지 대상 노드 A, B에 대해 X->A, X->B 엣지가 존재하고 B가 대표 노드일 때
- **THEN** X->A를 X->B로 재연결하면 중복이므로 X->A를 삭제한다

#### Scenario: self-loop 엣지 제거
- **WHEN** 머지 전 A->B 엣지가 있었고 A, B가 같은 대표 노드로 머지될 때
- **THEN** self-loop 엣지는 삭제된다

#### Scenario: 이미 머지된 사이트에 재실행
- **WHEN** 머지가 완료된 사이트에 대해 머지를 다시 실행할 때
- **THEN** 추가 변경 없이 완료된다 (멱등성)

---

### Requirement: Pioneer가 DB 기존 노드와 무관하게 BFS 큐를 관리한다
Pioneer의 BFS 큐에 링크를 넣을지 여부는 오직 `visited` 맵(인메모리, 세션 스코프)으로 결정해야 한다(SHALL). `CreateNode`가 duplicate key를 반환해도 해당 링크를 큐에 넣어야 한다(SHALL).

#### Scenario: DB에 이미 있는 노드의 자식 링크도 큐에 추가
- **WHEN** 루트 페이지에서 발견된 링크의 `CreateNode`가 duplicate key를 반환할 때
- **THEN** 해당 링크는 `visited` 맵에 등록되고 BFS 큐에 push된다

#### Scenario: visited 맵에 있는 링크는 큐에 추가하지 않음
- **WHEN** 이번 세션에서 이미 `visited` 맵에 등록된 링크를 다시 발견할 때
- **THEN** 해당 링크는 큐에 push되지 않는다 (edge만 생성)

#### Scenario: 재실행 시 depth 1 이상 탐색
- **WHEN** DB에 이전 크롤 데이터가 있는 상태에서 Pioneer를 재실행할 때
- **THEN** 루트의 자식 노드가 큐에 들어가고, 그 자식의 자식까지 BFS 탐색이 계속된다

---

### Requirement: MaxNodesPerSite는 새로 생성된 노드만 집계한다
`nodesProcessed` 카운터는 `CreateNode`가 성공하여 새 노드가 생성된 경우에만 증가해야 한다(SHALL). 기존 노드 재방문은 카운트하지 않아야 한다(SHALL).

#### Scenario: 기존 노드 재방문 시 카운터 미증가
- **WHEN** 큐에서 꺼낸 URL이 이미 DB에 존재하는 노드일 때
- **THEN** `nodesProcessed` 카운터가 증가하지 않는다

#### Scenario: 새 노드 생성 시 카운터 증가
- **WHEN** `CreateNode`가 성공하여 새 노드가 DB에 생성될 때
- **THEN** `nodesProcessed` 카운터가 1 증가한다

#### Scenario: quota가 새 노드 기준으로 동작
- **WHEN** DB에 기존 노드 50개가 있고 `MaxNodesPerSite=100`으로 재실행할 때
- **THEN** 기존 50개 재방문 후에도 quota가 남아 새 노드를 최대 100개까지 추가 생성할 수 있다

---

### Requirement: 크롤 완료 후 stale edge를 삭제한다
Pioneer는 크롤 완료 후, 이번 세션에서 방문한 노드의 outgoing edge 중 재확인되지 않은 edge를 삭제해야 한다(SHALL). 미방문 노드의 edge는 삭제하지 않아야 한다(SHALL).

#### Scenario: 사이트에서 링크가 제거된 경우
- **WHEN** 이전 크롤에서 A→B edge가 존재했으나 이번 크롤에서 A 페이지에 B 링크가 없을 때
- **THEN** A→B edge가 DB에서 삭제된다

#### Scenario: 방문하지 않은 노드의 edge는 유지
- **WHEN** `MaxNodesPerSite` 한도로 BFS가 중단되어 노드 C를 방문하지 못했을 때
- **THEN** 노드 C의 outgoing edge(C→D, C→E)는 삭제되지 않는다

#### Scenario: 재확인된 edge는 유지
- **WHEN** 이번 크롤에서 A→B edge가 다시 발견될 때 (CreateEdge 성공 또는 duplicate)
- **THEN** A→B edge는 삭제 대상에서 제외된다

---

### Requirement: URL 패턴으로 페이지 타입을 분류한다
시스템은 발견한 URL을 키워드 매칭을 통해 타입(list, detail)으로 분류해야 한다(SHALL). 타입 분류 기능은 순수한 타입 분류만 수행하며, 불필요한 URL 제외는 필터링 단계에서 처리해야 한다(SHALL). 정규식 패턴은 패키지 초기화 시 한 번만 컴파일해야 한다(SHALL).

#### Scenario: list 페이지 분류
- **WHEN** URL이 listing 키워드(trending, popular, hot, featured, recent, explore), gallery 키워드(gallery, collection, album, showcase), 또는 category 키워드(category, tag, genre, style, contest, event)를 포함할 때
- **THEN** 노드 타입이 'list'로 설정된다

#### Scenario: detail 페이지 분류 — 경로 키워드
- **WHEN** URL 경로가 'artworks', 'photos', 'works', 'illust' 등 콘텐츠 단수형 키워드를 포함할 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: detail 페이지 분류 — 경로 숫자 ID
- **WHEN** URL 경로에 4자리 이상 숫자 ID 패턴이 포함될 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: detail 페이지 분류 — 쿼리 파라미터
- **WHEN** URL 쿼리 파라미터에 명시적 ID 키(id, illust_id, artwork_id, photo_id)의 숫자 값이 포함될 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: 타입 분류는 skip을 반환하지 않는다
- **WHEN** 이전에 skip으로 분류되던 URL(ad, popup, login, signup, cart, checkout)을 타입 분류에 전달할 때
- **THEN** skip 타입을 반환하지 않고 기본값인 'list'를 반환한다

#### Scenario: 불필요한 URL 제외는 필터링 단계에서 처리
- **WHEN** URL이 'ad', 'popup', 'login', 'signup', 'cart', 'checkout'을 포함할 때
- **THEN** 해당 URL은 BFS 큐에 추가되지 않고 그래프에 포함되지 않는다

---

### Requirement: BFS로 사이트를 탐색한다 (Pioneer)
시스템은 너비 우선 탐색으로 사이트 링크를 순회해야 한다(SHALL). 링크 추출은 DOM 기반 파서를 사용하여 DOM 위치 정보를 함께 추출해야 하며(SHALL), 링크 필터링은 필터 체인을 통해 수행해야 한다(SHALL).

#### Scenario: DOM 기반 링크 추출
- **WHEN** Pioneer가 HTML 페이지에서 링크를 추출할 때
- **THEN** DOM 기반 파서를 사용하여 링크 URL과 DOM 위치 정보(태그, ID, 클래스)를 함께 추출한다

#### Scenario: 필터 체인을 통한 링크 필터링
- **WHEN** 추출된 링크 목록을 필터링할 때
- **THEN** 도메인, 확장자, 경로 패턴, 중복 필터를 순차적으로 적용한다

#### Scenario: 이미 방문한 링크에 대한 엣지 생성
- **WHEN** 필터링 과정에서 이미 방문한 링크가 감지되었을 때
- **THEN** 유효한 노드 ID가 있는 방문 링크에 대해 현재 노드에서 해당 노드로의 엣지를 생성한다

#### Scenario: 복합 우선순위 계산
- **WHEN** 신규 링크를 BFS 큐에 추가할 때
- **THEN** 노드 타입 기반 우선순위에 DOM 위치 기반 보정값을 더한 복합 우선순위를 적용한다

#### Scenario: 최대 노드 수 제한 준수
- **WHEN** Pioneer가 사이트를 탐색할 때
- **THEN** 사이트당 최대 노드 수 제한을 초과하지 않는다

#### Scenario: 부모 관계 추적 및 엣지 생성
- **WHEN** 새로운 노드를 발견할 때
- **THEN** 현재 노드에서 발견된 노드로의 엣지를 생성하여 부모-자식 관계를 추적한다

<!-- 기존 baseline의 Fetcher 인터페이스/FileFetcher/HTTPFetcher 시나리오 3개는 Pioneer가 아닌 crawler.BFSCrawler의 행위이므로 이 requirement에서 제거. crawler 패키지 자체의 spec으로 이관 필요 시 별도 change로 처리. -->

---

### Requirement: Harvester가 실제 HTML을 가져온다
시스템은 Harvester가 크롤 그래프의 노드 URL에서 실제 HTML을 가져올 수 있어야 한다(SHALL).

#### Scenario: Harvester HTML 가져오기
- **WHEN** Harvester가 노드의 sample_url 또는 template url로 HTML을 요청할 때
- **THEN** Pioneer와 동일한 HTTP 설정(사이즈 제한, 리다이렉트 제한, 타임아웃, User-Agent)으로 HTML을 가져온다

#### Scenario: Pioneer와 Harvester의 fetch 로직 공유
- **WHEN** Pioneer와 Harvester가 HTML을 가져올 때
- **THEN** 동일한 공유 함수를 사용하여 중복 구현을 방지한다

---

### Requirement: AI CLI 클라이언트가 비인터랙티브 모드로 동작한다
CLI 클라이언트가 `codex` 명령을 사용할 때, 비인터랙티브 서브커맨드를 자동으로 적용하여 TTY 없이 stdin으로 프롬프트를 전달할 수 있어야 한다(SHALL).

#### Scenario: codex 명령에 비인터랙티브 모드 자동 적용
- **WHEN** CLI 클라이언트의 command가 "codex"이고 Call을 실행할 때
- **THEN** 명령이 비인터랙티브 모드로 실행되며 stdin에서 프롬프트를 읽는다

#### Scenario: codex가 아닌 커스텀 명령에는 모드를 변경하지 않음
- **WHEN** CLI 클라이언트의 command가 "codex"가 아닌 다른 값일 때
- **THEN** 명령 args를 변경하지 않고 그대로 실행한다

#### Scenario: 기존 args가 있을 때 비인터랙티브 모드 인자가 우선 적용
- **WHEN** CLI 클라이언트의 command가 "codex"이고 추가 args가 설정되어 있을 때
- **THEN** 비인터랙티브 모드 인자가 기존 인자보다 우선 적용된다

---

### Requirement: show-map 시각화가 저장된 스크립트 기반으로 구현 상태를 판정한다
그래프 시각화에서 노드의 스크립트 구현 상태(HasScript)는 저장된 스크립트 데이터를 기반으로 판정해야 한다(SHALL). 파일 시스템을 조회하지 않아야 한다(SHALL).

#### Scenario: 스크립트가 존재하는 노드의 HasScript 판정
- **WHEN** 해당 사이트와 노드 타입 조합에 대한 스크립트가 저장소에 존재할 때
- **THEN** 해당 노드의 HasScript가 true로 설정된다

#### Scenario: 스크립트가 없는 노드의 HasScript 판정
- **WHEN** 해당 사이트와 노드 타입 조합에 대한 스크립트가 저장소에 존재하지 않을 때
- **THEN** 해당 노드의 HasScript가 false로 설정된다

---

### Requirement: JavaScript 파싱 스크립트를 실행하여 콘텐츠 항목을 추출한다
스크립트 실행기는 DB에 저장된 JavaScript 스크립트를 실행하여 HTML에서 콘텐츠 항목 배열을 반환해야 한다(SHALL). 스크립트 실행 중 에러가 발생하면 빈 배열과 에러를 반환해야 한다(SHALL).

#### Scenario: 정상적인 스크립트 실행
- **WHEN** 유효한 JavaScript 스크립트와 HTML이 주어질 때
- **THEN** 스크립트가 실행되어 추출된 콘텐츠 항목 배열이 반환된다

#### Scenario: 스크립트 구문 에러
- **WHEN** 문법 오류가 있는 JavaScript 스크립트가 주어질 때
- **THEN** 빈 배열과 구문 에러를 포함한 error가 반환된다

#### Scenario: 스크립트 런타임 에러
- **WHEN** 실행 중 예외가 발생하는 스크립트가 주어질 때
- **THEN** 빈 배열과 런타임 에러를 포함한 error가 반환된다

#### Scenario: 빈 HTML 입력
- **WHEN** 빈 문자열의 HTML이 주어질 때
- **THEN** 스크립트 실행이 정상 시도된다. 스크립트가 빈 배열을 반환하면 정상 처리되고, 예외를 throw하면 런타임 에러로 처리된다

#### Scenario: 스크립트 실행 타임아웃
- **WHEN** 스크립트가 지정된 타임아웃을 초과하여 실행될 때
- **THEN** 실행이 중단되고 빈 배열과 타임아웃 에러를 포함한 error가 반환된다

---

### Requirement: DOM 헬퍼 함수를 스크립트 런타임에 주입한다
실행 런타임은 스크립트가 HTML을 탐색할 수 있도록 DOM 유사 API를 제공해야 한다(SHALL). 최소한 querySelectorAll, querySelector, textContent, getAttribute 접근을 지원해야 한다(SHALL).

#### Scenario: querySelectorAll로 요소 목록 조회
- **WHEN** 스크립트가 CSS selector를 인자로 querySelectorAll을 호출할 때
- **THEN** 매칭되는 모든 요소의 배열이 반환된다

#### Scenario: querySelector로 단일 요소 조회
- **WHEN** 스크립트가 CSS selector를 인자로 querySelector를 호출할 때
- **THEN** 첫 번째 매칭 요소가 반환되거나, 없으면 null이 반환된다

#### Scenario: 요소의 textContent 접근
- **WHEN** 스크립트가 조회된 요소의 textContent를 읽을 때
- **THEN** 해당 요소의 텍스트 내용이 반환된다

#### Scenario: 요소의 getAttribute 접근
- **WHEN** 스크립트가 조회된 요소의 getAttribute를 호출할 때
- **THEN** 해당 속성 값이 반환되거나, 없으면 null이 반환된다

---

### Requirement: 스크립트 실행 결과를 콘텐츠 항목 배열로 변환한다
스크립트의 반환값을 콘텐츠 항목 배열로 변환해야 한다(SHALL). 필수 필드(title, mediaURL, mediaType)가 누락된 항목은 건너뛰어야 한다(SHALL). 선택 필드(description, sourceURL)가 빈 문자열이거나 누락된 경우에는 항목을 건너뛰지 않고 정상 처리해야 한다(SHALL).

#### Scenario: 정상적인 결과 변환
- **WHEN** 스크립트가 title, mediaURL, mediaType 필드를 포함한 객체 배열을 반환할 때
- **THEN** 각 객체가 콘텐츠 항목으로 변환되어 반환된다

#### Scenario: 필수 필드 누락 항목 스킵
- **WHEN** 스크립트 반환값 중 title, mediaURL, 또는 mediaType이 빈 문자열이거나 누락(undefined/null)인 항목이 있을 때
- **THEN** 해당 항목은 결과에서 제외된다

#### Scenario: sourceURL 누락 시 기본값 사용
- **WHEN** 추출된 항목에 sourceURL 필드가 없거나 빈 문자열일 때
- **THEN** 스크립트 실행 시 제공된 URL이 sourceURL로 사용된다

#### Scenario: 비배열 반환값 처리
- **WHEN** 스크립트가 배열이 아닌 값을 반환할 때 (undefined, null, 문자열, 숫자, 단일 객체 등)
- **THEN** 빈 배열과 에러가 반환된다

---

### Requirement: 콘텐츠 항목 중복 체크
처리 파이프라인은 봇이 이미 저장한 콘텐츠와 동일한 sourceURL을 가진 항목을 중복으로 판단하여 건너뛰어야 한다(SHALL). 중복 판단은 봇 계정이 생성한 Pin만을 대상으로 해야 한다(SHALL). 중복 건수를 집계하여 반환해야 한다(SHALL).

#### Scenario: 신규 콘텐츠 통과
- **WHEN** sourceURL에 해당하는 봇 Pin이 DB에 존재하지 않는 항목이 입력될 때
- **THEN** 해당 항목은 중복이 아닌 것으로 판단되어 다음 단계로 진행된다

#### Scenario: 봇이 이미 수집한 콘텐츠 중복 스킵
- **WHEN** sourceURL에 해당하는 봇 Pin이 이미 DB에 존재하는 항목이 입력될 때
- **THEN** 해당 항목은 건너뛰고 중복 카운트가 증가한다

#### Scenario: 일반 사용자 Pin과는 중복 판정하지 않음
- **WHEN** 일반 사용자가 이미 동일 sourceURL로 Pin을 생성했지만 봇 Pin은 없을 때
- **THEN** 해당 항목은 중복이 아닌 것으로 판단되어 정상 처리된다

#### Scenario: 같은 배치 내 중복 처리
- **WHEN** 동일 sourceURL을 가진 항목이 같은 배치에 2개 이상 포함될 때
- **THEN** 첫 번째만 처리하고 나머지는 중복으로 처리된다

---

### Requirement: 미디어 파일을 스토리지에 다운로드하여 저장한다
처리 파이프라인은 항목의 mediaURL에서 미디어 파일을 다운로드하여 스토리지에 업로드해야 한다(SHALL). 업로드된 파일의 경로를 Pin 생성에 사용해야 한다(SHALL).

#### Scenario: 미디어 다운로드 및 업로드
- **WHEN** 지원되는 mediaType(image, audio, video)의 항목이 처리될 때
- **THEN** mediaURL에서 미디어를 다운로드하여 스토리지에 업로드하고 저장 경로를 반환한다

#### Scenario: 미디어 다운로드 또는 업로드 실패 시 해당 항목 스킵
- **WHEN** 미디어 다운로드 또는 스토리지 저장 과정에서 에러가 발생할 때 (404, timeout, 크기 초과, MIME 미지원, 네트워크 오류 등)
- **THEN** 해당 항목은 건너뛰고 에러가 로그에 기록되며 실패 카운트가 증가한다

---

### Requirement: Pin을 DB에 생성한다
처리 파이프라인은 중복 체크와 미디어 저장을 통과한 항목에 대해 Pin을 생성해야 한다(SHALL). 생성된 Pin은 시스템 봇 계정 소유여야 한다(SHALL). 생성된 Pin 수를 반환해야 한다(SHALL).

#### Scenario: 정상적인 Pin 생성
- **WHEN** 중복이 아니고 미디어 다운로드가 성공한 항목이 있을 때
- **THEN** Pin이 시스템 봇 계정 소유로 DB에 생성되고 생성 카운트가 증가한다

#### Scenario: Pin 생성 실패 시 해당 항목 스킵
- **WHEN** DB 에러로 Pin 생성이 실패할 때
- **THEN** 해당 항목은 건너뛰고 에러가 로그에 기록되며 실패 카운트가 증가하고 나머지 항목 처리는 계속된다

---

### Requirement: 처리 파이프라인이 배치 처리 통계를 반환한다
처리 파이프라인은 한 노드에서 추출된 항목 배열을 받아 처리한 뒤, 한 번의 호출 결과로 생성 건수, 중복 건수, 실패 건수를 구분하여 반환해야 한다(SHALL).

#### Scenario: 혼합 결과 통계
- **WHEN** 5개 항목 중 2개가 중복이고 1개가 다운로드 실패이고 2개가 신규일 때
- **THEN** 생성=2, 중복=2, 실패=1이 반환된다

#### Scenario: 전체 중복 시 통계
- **WHEN** 모든 항목이 중복일 때
- **THEN** 생성=0, 실패=0이고 중복이 전체 건수와 같다

---

### Requirement: Harvester 실행 완료 시 전체 통계를 집계한다
Harvester는 모든 노드의 처리가 끝난 후, 노드별 파이프라인 결과를 누적하여 전체 통계(총 처리 노드 수, 총 Pin 생성 수, 총 중복 수, 총 실패 수)를 반환해야 한다(SHALL).

#### Scenario: 다수 노드 처리 후 전체 통계
- **WHEN** 3개 노드를 처리하여 각각 생성=1/중복=2/실패=0, 생성=3/중복=0/실패=1, 생성=0/중복=5/실패=0일 때
- **THEN** 전체 통계는 노드수=3, 생성=4, 중복=7, 실패=1이 반환된다

---

### Requirement: Harvester CLI가 실제 모드를 지원한다
Harvester CLI는 설정에 따라 실제 스크립트 실행기와 처리 파이프라인을 사용하는 실제 모드를 지원해야 한다(SHALL). 실제 모드란 mock 대신 프로덕션 스크립트 실행기와 처리 파이프라인을 사용하여 실제 콘텐츠를 추출하고 Pin을 생성하는 모드를 의미한다. 기본 동작은 mock을 유지하여 기존 워크플로우에 영향을 주지 않아야 한다(SHALL). 실행 완료 후 통계를 로그로 출력해야 한다(SHALL).

#### Scenario: 실제 모드로 Harvester 실행
- **WHEN** 실제 모드가 활성화된 상태에서 Harvester를 실행할 때
- **THEN** 실제 스크립트 실행기와 처리 파이프라인이 사용되어 스크립트가 실행되고 Pin이 생성된다

#### Scenario: 기본 mock 모드 유지
- **WHEN** 실제 모드가 명시적으로 활성화되지 않은 상태에서 Harvester를 실행할 때
- **THEN** 기존과 동일하게 mock이 사용된다

#### Scenario: 인식되지 않는 설정 값은 mock으로 동작
- **WHEN** 실제 모드 설정에 인식할 수 없는 값이 주어질 때
- **THEN** mock 모드로 동작한다

#### Scenario: 실행 결과 통계 출력
- **WHEN** Harvester 실행이 완료될 때
- **THEN** 총 처리 노드 수, 생성된 Pin 수, 중복 스킵 수, 실패 수가 로그에 출력된다
