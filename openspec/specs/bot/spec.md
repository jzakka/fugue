## Purpose

`bot` capability는 Fugue의 콘텐츠 발견·추출 파이프라인을 정의한다. Pioneer는 사이트 그래프를 BFS로 탐색하며 페이지 템플릿 패턴 단위로 노드/엣지를 저장하고 노드 타입별 파싱 스크립트를 보유한다. Harvester는 그 그래프를 따라 실제 콘텐츠를 추출해 핀으로 적재한다. 본 스펙은 이 두 행위자의 외부 관찰 가능 동작(그래프 정규화 규칙, 스크립트 검증·저장, 페이지 fetch·재시도, 스냅샷 저장, primary 이미지 캐시 TTL/만료)을 행위 계약으로 기술한다.
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
DomainFilter는 단일 루트 도메인 비교가 아니라 **Allow 키워드 리스트**와 **Deny 키워드 리스트**를 받아 링크 호스트에 대한 substring 매칭으로 필터링해야 한다(SHALL). 종전의 "루트 도메인 고정 비교" 정책은 본 요구사항이 대체하며, 교차 사이트 크롤을 기본 허용한다(SHALL).

- Deny 리스트에 매칭되는 호스트는 항상 차단한다(SHALL).
- Allow 리스트가 비어 있으면 Deny에 걸리지 않은 모든 호스트를 통과시킨다(SHALL). 즉 **교차 사이트 크롤을 기본 허용**한다.
- Allow 리스트가 비어 있지 않으면, Allow 키워드 중 하나라도 매칭되는 호스트만 통과시킨다(SHALL).
- 매칭 규칙은 호스트 문자열을 lowercased + `www.` 접두어 제거 후 **대소문자 무시 substring** 비교다(SHALL).
- 국가별 TLD에 대한 특별 처리는 없으며 substring 매칭이 그대로 적용된다(SHALL).

#### Scenario: Allow 비어 있음 - 교차 사이트 기본 허용
- **WHEN** Allow=[], Deny=[] 이고 seed 호스트와 다른 외부 호스트 링크가 입력될 때
- **THEN** 해당 링크는 필터를 통과한다

#### Scenario: Deny 리스트 매칭 차단
- **WHEN** Deny 키워드가 "adnetwork"이고 호스트에 "adnetwork"를 포함하는 링크가 입력될 때
- **THEN** 해당 링크는 필터에서 제거된다

#### Scenario: Allow 리스트가 있으면 화이트리스트 모드
- **WHEN** Allow 키워드가 "music"이고 호스트가 "music.io"인 링크가 입력될 때
- **THEN** 해당 링크는 Allow 매칭으로 통과한다
- **WHEN** 같은 Allow 설정에서 호스트가 "other.net"인 링크가 입력될 때
- **THEN** 해당 링크는 Allow에 매칭되지 않아 제거된다

#### Scenario: Deny가 Allow보다 우선
- **WHEN** Allow 키워드 "example.com"과 Deny 키워드 "tracker"가 모두 설정되고, 호스트 "tracker.example.com" 링크가 입력될 때
- **THEN** Deny 매칭이 우선 적용되어 해당 링크는 제거된다

#### Scenario: www 접두어 및 대소문자 무시
- **WHEN** Allow 키워드 "example.com"이 설정되고 호스트 "WWW.Example.com" 링크가 입력될 때
- **THEN** www와 대소문자가 정규화되어 Allow 매칭으로 통과한다

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
URL 정규화는 다음을 **모두** 수행해야 한다(SHALL). 종전 규칙(트래킹 파라미터 제거, www 제거, trailing slash 통일)에 scheme 소문자화, default port 제거, query 파라미터 이름순 정렬, fragment 제거를 추가하여 RFC 3986 수준의 정규화를 달성한다.

- scheme을 소문자로 변환한다(SHALL).
- host를 소문자로 변환하고 `www.` 접두어를 제거한다(SHALL).
- default port를 제거한다: `http`의 `:80`과 `https`의 `:443`(SHALL). non-default 포트는 보존한다(SHALL).
- fragment(`#...`)를 제거한다(SHALL).
- 루트가 아닌 경로의 trailing slash를 제거한다(SHALL).
- 트래킹 파라미터를 제거한다: `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`, `ref`, `fbclid`, `gclid`(SHALL).
- 남은 query 파라미터를 이름(key) 오름차순으로 정렬하여 재인코딩한다(SHALL).
- 경로(path)의 대소문자는 **보존**한다(SHALL).

#### Scenario: scheme과 host는 소문자, 경로는 보존
- **WHEN** `HTTPS://Example.COM/Page` 형태의 URL이 입력될 때
- **THEN** `https://example.com/Page` 로 정규화된다

#### Scenario: default port 제거
- **WHEN** http 스킴의 URL에 기본 포트(80)가 명시적으로 포함될 때
- **THEN** 정규화된 URL은 기본 포트를 포함하지 않는다
- **WHEN** https 스킴의 URL에 기본 포트(443)가 명시적으로 포함될 때
- **THEN** 정규화된 URL은 기본 포트를 포함하지 않는다

#### Scenario: non-default 포트는 보존
- **WHEN** URL이 해당 스킴의 기본 포트가 아닌 포트(예: 8080)를 포함할 때
- **THEN** 정규화된 URL은 해당 포트를 그대로 유지한다

#### Scenario: query 파라미터 이름순 정렬
- **WHEN** 파라미터가 `b=2&a=1&c=3` 순서로 입력될 때
- **THEN** 정규화된 URL은 `a=1&b=2&c=3` 순서가 된다

#### Scenario: 트래킹 파라미터 제거 후 정렬
- **WHEN** `utm_source=twitter&id=123&a=z` 쿼리가 입력될 때
- **THEN** `utm_source`가 제거되고 남은 파라미터가 `a=z&id=123`로 정렬된다

#### Scenario: fragment 제거
- **WHEN** URL에 `#section` 등 fragment가 포함될 때
- **THEN** fragment가 제거된 URL이 반환된다

#### Scenario: trailing slash 통일과 루트 보존
- **WHEN** 경로가 `/page/`인 URL이 입력될 때
- **THEN** trailing slash가 제거되어 `/page`가 된다
- **WHEN** 경로가 루트 `/`인 URL이 입력될 때
- **THEN** 루트 `/`는 보존된다

#### Scenario: 대표 복합 케이스
- **WHEN** `http://Example.com:80/path/?b=2&a=1#frag` 가 입력될 때
- **THEN** `http://example.com/path?a=1&b=2` 로 정규화된다

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
Pioneer의 기본 필터 체인은 다음 고정 순서를 따라야 한다(SHALL): (1) Domain allow/deny, (2) Extension, (3) PathPattern, (4) Robots, (5) Dedup. 각 필터의 출력이 다음 필터의 입력이 되며(SHALL), 이 순서는 값이 싼 필터를 앞에, 네트워크 I/O(Robots)와 공유 맵/DB 조회(Dedup)가 있는 값비싼 필터를 뒤에 배치하기 위해 고정되어야 한다(SHALL).

- Domain / Extension / PathPattern은 인메모리 문자열 또는 regex 매칭으로 가장 저렴하다.
- Robots는 캐시 hit 시 인메모리, miss 시 HTTP GET이 발생하므로 앞 세 필터로 먼저 후보를 줄인 뒤 평가한다.
- Dedup은 공유 visited 맵 조회와 canonical 해시 계산이 필요하므로 가장 뒤에 배치된다.
- Robots 필터는 **Enqueue 단계**에서 호출되어 의미적 차단을 담당한다. **Claim 단계**의 host token bucket 체크는 scheduler-host-token-bucket capability가 담당하며 본 스펙과 별개다.

#### Scenario: 기본 체인 구성
- **WHEN** Pioneer가 필터 체인을 기본 구성으로 초기화할 때
- **THEN** 체인의 필터 순서는 Domain → Extension → PathPattern → Robots → Dedup이다

#### Scenario: 앞 단계에서 탈락한 링크는 뒤 단계에 도달하지 않는다
- **WHEN** 어떤 링크가 Extension 또는 PathPattern 필터에서 제거될 때
- **THEN** 해당 링크는 Robots나 Dedup에 도달하지 않으며, robots.txt 조회 비용을 유발하지 않는다

#### Scenario: 빈 링크 목록 처리
- **WHEN** 빈 링크 목록이 필터 체인에 입력될 때
- **THEN** 에러 없이 빈 목록이 반환되며 Robots는 robots.txt를 fetch하지 않는다

#### Scenario: Enqueue 단계와 Claim 단계의 책임 분리
- **WHEN** Pioneer가 링크를 Enqueue하고 scheduler가 해당 URL을 Claim할 때
- **THEN** Enqueue 단계에서 Robots 필터가 Disallow를 거르고, Claim 단계에서 host token bucket이 속도를 제어한다

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

### Requirement: Harvester가 실제 HTML을 가져온다

시스템은 Harvester가 크롤 그래프의 노드 URL에서 실제 HTML을 가져올 수 있어야 한다(SHALL). 본 Requirement 블록의 Scenario 집합은 기존 `bot` capability의 동명 Requirement가 보유하던 Scenario 2건을 다음과 같이 대체한다(근거: design.md Decision 1b, 1c). (a) 기존 "Harvester HTML 가져오기" Scenario는 본 블록의 "스냅샷 hit 시 네트워크 호출 없이 로컬 파싱" 및 "출처 무관한 응답 의미론" Scenario로 대체되며, (b) 기존 "Pioneer와 Harvester의 fetch 로직 공유" Scenario는 본 블록의 "Pioneer와 Harvester의 HTTP 경계 설정 공유" Scenario로 **완화 대체**된다(공유 범위가 "동일 fetch 함수"에서 "HTTP 경계 설정 helper"로 축소됨). Harvester는 단일 fetch 진입점에 의존하며, 해당 진입점은 **ObjectStorage 스냅샷을 우선 시도하고 사용 불가 시 HTTP fetch로 폴백**하는 합성 의미론을 제공해야 한다(SHALL). 진입점이 반환하는 바이트열은 출처(스냅샷/HTTP)와 무관하게 동일한 의미의 원본 HTML로 후속 파싱 파이프라인에 전달되어야 한다(SHALL). 참고(informative) — 설계 의사코드: `apps/api/fuguebot_pseudo.go` 라인 97–112. 의사코드의 구체 타입 이름은 구현 세부이며 본 Requirement의 행위 계약 대상이 아니다.

ObjectStorage 조회 시 사용하는 스냅샷 키 포맷과 해시 함수는 본 Requirement에서 자체 정의하지 않고, **동일 `bot` capability 내부의 스냅샷 쓰기 경로(구 `pioneer-snapshot-storage` change에서 확정, 현재는 `bot` capability의 스냅샷 쓰기 Requirement로 존재)가 확정한 공용 키 규약을 그대로 따른다**(SHALL). Harvester 측에서 키 포맷·해시 함수를 재구현해서는 안 된다(MUST NOT).

#### Scenario: 스냅샷 hit 시 네트워크 호출 없이 로컬 파싱
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청하고 ObjectStorage에서 유효한 스냅샷이 반환될 때
- **THEN** Harvester는 외부 사이트로 HTTP 요청을 보내지 않고, ObjectStorage에서 받은 본문만으로 파싱 파이프라인을 진행한다

#### Scenario: 출처 무관한 응답 의미론
- **WHEN** Harvester가 ObjectStorage 또는 HTTP 어느 쪽에서든 본문을 받을 때
- **THEN** 후속 파서/스크립트 실행기는 출처를 구분하지 않고 동일한 의미의 원본 HTML 바이트열을 관찰한다. 저장 경로에 적용된 모든 저장 포맷 변환은 fetch 경계 안에서 완결되어야 하며, 호출자에게 저장 포맷의 세부(예: 압축 등)가 노출되지 않는다

#### Scenario: Pioneer와 Harvester의 HTTP 경계 설정 공유
- **WHEN** Pioneer가 원본을 fetch하거나 Harvester가 스냅샷 사용 불가로 HTTP 폴백 경로로 HTML을 가져올 때(즉 어느 쪽이든 HTTP를 실제로 호출하는 시점)
- **THEN** 동일한 HTTP 경계 설정(사이즈 제한, 리다이렉트 제한, 타임아웃, User-Agent)을 공유하여 중복 구현과 드리프트를 방지한다. 상위 fetch 인터페이스 시그니처까지 동일해야 하는 것은 아니며, HTTP helper 수준의 공유만을 요구한다

#### Scenario: 스냅샷 키 규약은 동일 capability의 쓰기 경로를 참조한다
- **WHEN** Harvester의 ObjectStorage 조회가 스냅샷 키를 계산할 때
- **THEN** 동일 `bot` capability 내부의 스냅샷 쓰기 Requirement가 확정한 공용 키 빌더(normalized URL 해시 기반)를 그대로 사용하며, 본 Requirement에서 키 포맷을 별도로 정의하지 않는다. 또한 키 빌더 입력으로 전달하는 normalized URL은 쓰기 경로가 사용한 URL 정규화 규칙과 동일한 규칙의 결과여야 한다(읽기 키와 쓰기 키가 비트 단위로 일치하도록 보장한다)

#### Scenario: 스냅샷 조회 시간 기준
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청할 때
- **THEN** 스냅샷 키의 시간 세그먼트는 Harvester가 해당 fetch 요청을 수행하는 시점(호출 단위로 관찰되는 현재 UTC 날짜)로 결정되며, 스냅샷 쓰기 경로가 같은 UTC 일자 내에 쓴 스냅샷만 hit 대상이 된다. 그 외(과거 일자에 쓰인 스냅샷 등)는 "사용 불가"로 간주되어 HTTP 폴백 경로로 수렴한다

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
스크립트 실행기는 DB에 저장된 JavaScript 스크립트를 실행하여 HTML에서 콘텐츠 항목 배열을 반환해야 한다(SHALL). 스크립트 실행 중 에러가 발생하면 빈 배열과 에러를 반환해야 한다(SHALL). **본 경로는 default HTML→Pin 변환 경로가 아니라 per-site override 경로다(SHALL).** Default 변환은 `harvester` capability의 generic HTML→Pin extractor가 담당하며, 스크립트 실행기는 `harvester` capability가 정의한 `PerSiteAdapter` 인터페이스의 한 가지 구현(`ScriptAdapter`)으로만 호출되어야 한다(SHALL). 어떤 사이트에 대해 ScriptAdapter가 등록되어 있지 않거나 실행이 실패하면 시스템은 generic extractor로 fallback해야 한다(SHALL).

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

#### Scenario: 스크립트 경로는 default가 아니라 per-site override
- **WHEN** 어떤 사이트의 HTML이 default 변환 경로(generic HTML→Pin extractor)로도 처리 가능할 때
- **THEN** 시스템은 해당 사이트에 대해 `PerSiteAdapter`(예: ScriptAdapter)가 명시적으로 등록되어 있을 때만 스크립트 실행 경로를 사용하고, 그렇지 않으면 generic extractor를 사용한다

#### Scenario: ScriptAdapter 실패 시 generic extractor로 fallback
- **WHEN** ScriptAdapter로 래핑된 스크립트 실행기가 타임아웃, 구문 에러, 런타임 에러 등 어떤 사유로든 실패하여 빈 배열과 에러를 반환할 때
- **THEN** Harvester는 같은 HTML에 대해 generic HTML→Pin extractor로 fallback하여 PinDocument 생성을 시도한다

---

### Requirement: DOM 헬퍼 함수를 스크립트 런타임에 주입한다
실행 런타임은 스크립트가 HTML을 탐색할 수 있도록 DOM 유사 API를 제공해야 한다(SHALL). 최소한 querySelectorAll, querySelector, textContent, getAttribute 접근을 지원해야 한다(SHALL). **본 헬퍼는 `PerSiteAdapter`의 ScriptAdapter 구현 내부에서만 사용되며, default 변환 경로(generic HTML→Pin extractor)는 표준 HTML 파서를 사용해야 한다(SHALL).**

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

#### Scenario: DOM 헬퍼는 default 경로에서 사용되지 않는다
- **WHEN** Harvester가 generic HTML→Pin extractor를 통해 페이지를 처리할 때
- **THEN** 시스템은 JavaScript DOM 헬퍼 런타임을 초기화하지 않고 표준 HTML 파서를 사용한다

---

### Requirement: 스크립트 실행 결과를 콘텐츠 항목 배열로 변환한다
스크립트의 반환값을 콘텐츠 항목 배열로 변환해야 한다(SHALL). 필수 필드(title, mediaURL, mediaType)가 누락된 항목은 건너뛰어야 한다(SHALL). 선택 필드(description, sourceURL)가 빈 문자열이거나 누락된 경우에는 항목을 건너뛰지 않고 정상 처리해야 한다(SHALL). **본 변환 결과는 그대로 Pin 다건으로 indexing되지 않는다(SHALL NOT). ScriptAdapter는 N개의 콘텐츠 항목을 `harvester` capability가 정의한 PinDocument 1건으로 축약하여, 첫 번째 항목을 정본 메타로 채택하고 나머지 항목들은 `og_data.media_candidates`에 추가해야 한다(SHALL).**

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

#### Scenario: N개 항목을 PinDocument 1건으로 축약
- **WHEN** ScriptAdapter가 한 페이지에 대해 N개의 콘텐츠 항목을 받을 때
- **THEN** 첫 번째 항목의 title/mediaURL/mediaType이 PinDocument의 정본 메타로 채택되고, 나머지 항목들은 type/url과 함께 `og_data.media_candidates`에 추가되어 노드 1개당 정확히 1건의 PinDocument가 반환된다

#### Scenario: 빈 결과 시 generic으로 fallback
- **WHEN** 스크립트 실행이 성공했으나 유효한 콘텐츠 항목 0건을 반환할 때
- **THEN** ScriptAdapter는 에러로 간주하여 generic HTML→Pin extractor로의 fallback이 일어나도록 한다

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

### Requirement: Pioneer는 fetch 성공 시 raw 응답을 object storage에 스냅샷 저장한다
스냅샷 저장 기능이 운영 토글로 활성화된 상태에서 Pioneer가 URL을 fetch하여 성공적으로 본문을 수신한 경우, 시스템은 해당 raw 응답 바이트를 object storage에 스냅샷으로 업로드해야 한다(SHALL). "fetch 성공"은 HTTP 2xx 응답 수신 및 본문 길이 > 0을 의미한다(SHALL). 운영 토글이 비활성화된 경우 시스템은 어떤 스냅샷도 업로드하지 않아야 한다(SHALL NOT).

#### Scenario: 2xx 본문 수신 시 스냅샷 업로드
- **GIVEN** 스냅샷 저장 기능이 활성화되어 있을 때
- **WHEN** Pioneer가 URL을 fetch하여 HTTP 200 응답과 비어 있지 않은 본문을 수신할 때
- **THEN** 해당 raw 응답 바이트를 object storage에 업로드한다

#### Scenario: 링크 추출과 별개로 저장 수행
- **GIVEN** 스냅샷 저장 기능이 활성화되어 있을 때
- **WHEN** Pioneer가 fetch 성공 후 링크를 추출할 때
- **THEN** 링크 추출과 무관하게 동일한 raw 바이트가 스냅샷으로 저장된다

#### Scenario: 스냅샷 저장 기능 비활성화 시 업로드 스킵
- **GIVEN** 스냅샷 저장 기능이 비활성화되어 있을 때
- **WHEN** Pioneer가 URL을 fetch하여 HTTP 2xx 응답과 비어 있지 않은 본문을 수신할 때
- **THEN** object storage에 스냅샷이 업로드되지 않으며, 링크 추출과 후속 큐 처리는 정상적으로 수행된다

---

### Requirement: 스냅샷은 gzip 압축으로 저장되며 TTL 365일을 따른다
시스템은 스냅샷을 gzip으로 압축하여 저장해야 한다(SHALL). 스냅샷의 보존 기간은 업로드 시점 기준 365일이며, 이 기간이 지나면 삭제되어야 한다(SHALL).

#### Scenario: 압축된 콘텐츠 업로드
- **WHEN** Pioneer가 raw 응답 바이트를 스냅샷으로 저장할 때
- **THEN** 바이트는 gzip으로 압축된 상태로 object storage에 업로드된다

#### Scenario: 365일 경과 후 만료
- **WHEN** 스냅샷이 업로드된 지 365일이 지났을 때
- **THEN** 해당 스냅샷은 object storage에서 삭제된다

---

### Requirement: 스냅샷 키는 normalized URL의 sha256 기반이다
시스템은 스냅샷 키를 normalized URL의 **sha256** digest(hex 인코딩 64자 소문자)를 기반으로 생성해야 한다(SHALL). 키 형식은 `snapshots/{sha256(normalized_url)}/{yyyymmdd}.html.gz` 이며, `{sha256(normalized_url)}`은 정확히 64자의 hex digest이고 `{yyyymmdd}`는 UTC 기준 fetch 날짜이다(SHALL). 동일한 normalized URL은 동일한 sha256 hex를 산출해야 한다(SHALL).

#### Scenario: 동일 URL은 동일 sha256
- **WHEN** Pioneer가 normalized 결과가 같은 두 URL을 각각 fetch할 때
- **THEN** 두 스냅샷 키의 sha256 hex 세그먼트가 동일하다

#### Scenario: 키 형식 준수 (64자 hex + UTC 날짜)
- **WHEN** Pioneer가 UTC 2026-04-17에 URL을 fetch하여 스냅샷을 저장할 때
- **THEN** 업로드되는 객체 키는 `snapshots/<64-char-sha256-hex>/20260417.html.gz` 형태이고, hex 세그먼트는 소문자 `[0-9a-f]`로만 구성된 정확히 64자다

#### Scenario: 같은 날 같은 URL 재fetch
- **WHEN** Pioneer가 같은 UTC 날짜에 동일한 normalized URL을 두 번 fetch하여 저장할 때
- **THEN** 두 번째 업로드는 첫 번째와 같은 키를 덮어쓴다

---

### Requirement: 동일 키에 대한 동시 쓰기는 last-write-wins이다
시스템은 동일 키에 대한 동시 PUT을 object storage의 기본 atomic PUT 동작에 위임해야 한다(SHALL). 애플리케이션 레벨의 lock, conditional write, versioning을 사용하지 않아야 한다(SHALL NOT). 마지막에 commit된 PUT이 최종 객체로 남아야 한다(SHALL).

#### Scenario: 동일 URL을 여러 Pioneer 워커가 같은 날 저장
- **WHEN** 두 개 이상의 Pioneer 워커가 동일한 normalized URL을 같은 UTC 날짜에 각각 fetch하여 스냅샷을 업로드할 때
- **THEN** 두 PUT 모두 동일 키를 대상으로 수행되며, 마지막으로 commit된 쓰기의 내용이 최종 객체로 유지된다 (last-write-wins)

#### Scenario: 동시 쓰기 시 별도 충돌 에러가 Pioneer에 전파되지 않음
- **WHEN** 동일 키에 대한 동시 PUT이 발생할 때
- **THEN** Pioneer는 lock 획득 실패나 version conflict 같은 별도 에러 경로를 거치지 않고, 각자의 PUT 결과(성공/실패)만 일반 업로드 경로로 처리한다

---

### Requirement: fetch 실패 시 스냅샷을 저장하지 않는다
시스템은 fetch가 실패한 경우 스냅샷을 업로드하지 않아야 한다(SHALL). 실패에는 HTTP 4xx/5xx 응답, 네트워크 오류, 타임아웃, 본문 길이 0이 포함된다(SHALL).

#### Scenario: HTTP 404 응답
- **WHEN** Pioneer가 fetch한 URL이 HTTP 404를 반환할 때
- **THEN** object storage에 스냅샷이 업로드되지 않는다

#### Scenario: 네트워크 타임아웃
- **WHEN** Pioneer가 fetch 중 타임아웃으로 실패할 때
- **THEN** object storage에 스냅샷이 업로드되지 않는다

#### Scenario: 본문이 비어 있는 성공 응답
- **WHEN** Pioneer가 HTTP 2xx를 수신했으나 본문 길이가 0일 때
- **THEN** object storage에 스냅샷이 업로드되지 않는다

---

### Requirement: 스냅샷 저장 실패는 fail-open으로 처리한다
시스템은 object storage 업로드가 실패하더라도 Pioneer의 크롤 진행을 중단시키지 않아야 한다(SHALL). 업로드 실패는 로그로 기록되어야 하며(SHALL), fetch된 응답으로부터 링크 추출 및 후속 큐 처리는 정상적으로 수행되어야 한다(SHALL).

#### Scenario: 업로드 실패 시 크롤 계속
- **WHEN** Pioneer가 fetch 성공 후 object storage에 업로드를 시도했으나 업로드가 실패할 때
- **THEN** Pioneer는 해당 URL의 링크 추출과 다음 URL 처리를 계속한다

#### Scenario: 업로드 실패 로그 기록
- **WHEN** object storage 업로드가 실패할 때
- **THEN** 실패 원인이 로그로 남는다

#### Scenario: 업로드 실패가 스케줄러 상태에 영향 없음
- **WHEN** object storage 업로드만 실패하고 fetch는 성공했을 때
- **THEN** URLScheduler 상 해당 URL은 fetch 성공으로 취급되어 재시도 큐에 들어가지 않는다

---

### Requirement: Pioneer는 ParseLinks 후 FilterLinks를 거쳐 Enqueue한다
Pioneer Run 루프에서 프런티어 큐에 Enqueue되는 URL은 **반드시 필터 체인의 최종 출력 집합에 속해야 한다**(SHALL). 필터 체인의 입력 URL은 `fetchHTML`이 반환하는 redirect chain의 **최종 URL**이어야 한다(SHALL).

#### Scenario: Enqueue된 URL은 필터 체인 통과 결과만 포함한다
- **WHEN** Pioneer가 한 페이지에서 추출한 링크 목록을 필터 체인에 투입하고 그 결과를 큐에 Enqueue한 직후 큐 상태를 관찰할 때
- **THEN** 해당 페이지로부터 유래한 모든 큐 항목은 필터 체인의 최종 출력 집합에 포함된다

#### Scenario: 빈 결과 처리
- **WHEN** 필터 체인이 모든 링크를 걸러내어 빈 목록을 반환할 때
- **THEN** Pioneer는 에러 없이 다음 Dequeue로 진행하며 해당 페이지로부터 Enqueue된 URL은 0건이다

#### Scenario: Redirect chain의 최종 URL만 사용
- **WHEN** Pioneer가 301/302 리디렉션을 거쳐 최종 페이지에 도달할 때
- **THEN** 필터 체인과 canonicalization은 최종 URL에만 적용되고 중간 redirect URL은 검사되지 않는다

#### Scenario: 파싱 불가능한 URL은 큐에 포함되지 않는다
- **WHEN** 추출된 링크 중 URL이 빈 문자열이거나 파싱 불가능하여 호스트를 얻을 수 없는 항목이 포함될 때
- **THEN** 해당 항목은 필터 체인에서 제거되어 큐에 Enqueue되지 않는다

---

### Requirement: RobotsFilter는 robots.txt를 존중하여 URL을 필터링한다
RobotsFilter는 필터 체인의 구성 요소로서(SHALL), 각 링크의 호스트에 대한 robots.txt를 조회하여 `FugueBot` User-agent(없으면 `*`)의 Disallow 규칙에 매칭되는 URL을 제거해야 한다(SHALL). User-agent 블록은 `FugueBot` 우선 사용, 없을 때 `*` fallback이며 두 블록을 병합하지 않는다(SHALL).

#### Scenario: Disallow에 매칭되면 차단
- **WHEN** 링크의 경로가 해당 호스트 robots.txt의 Disallow 규칙에 매칭될 때
- **THEN** 해당 링크는 필터에서 제거된다

#### Scenario: Allow/Disallow 모두 없으면 통과
- **WHEN** robots.txt에 `FugueBot` 또는 `*`에 대한 Disallow 규칙이 없을 때
- **THEN** 해당 링크는 필터를 통과한다

#### Scenario: FugueBot 규칙이 우선, 없으면 `*` fallback
- **WHEN** robots.txt에 `User-agent: FugueBot` 블록이 존재할 때
- **THEN** 해당 블록의 규칙을 사용하고 `*` 블록은 무시된다
- **WHEN** `FugueBot` 블록이 없을 때
- **THEN** `User-agent: *` 블록의 규칙을 사용한다

---

### Requirement: RobotsFilter는 lazy fetch와 호스트별 24시간 캐시를 사용한다
RobotsFilter는 호스트별로 robots.txt를 **최초 필요 시점에만 fetch**해야 하며(SHALL), 파싱 결과를 호스트별 인메모리 맵에 캐시해야 한다(SHALL). 캐시 TTL은 **24시간**이어야 하며(SHALL), TTL 경과 후 다음 접근에 재조회해야 한다(SHALL).

#### Scenario: 최초 접근 시 fetch
- **WHEN** 어떤 호스트에 대한 링크가 RobotsFilter에 처음 도달할 때
- **THEN** RobotsFilter는 `https://<host>/robots.txt`를 fetch하고 결과를 캐시한다

#### Scenario: 캐시 적중
- **WHEN** 같은 호스트에 대한 링크가 24시간 이내에 재도달할 때
- **THEN** RobotsFilter는 새로운 fetch 없이 캐시된 규칙을 사용한다

#### Scenario: TTL 만료 후 재조회
- **WHEN** 캐시 엔트리가 저장된 지 24시간을 초과한 뒤 같은 호스트의 링크가 도달할 때
- **THEN** RobotsFilter는 robots.txt를 다시 fetch하여 캐시를 갱신한다

---

### Requirement: RobotsFilter는 fetch 실패 시 fail-open한다
RobotsFilter는 robots.txt fetch가 네트워크 오류, 타임아웃, 5xx 응답 등으로 실패할 때 **모든 링크를 허용(fail-open)**해야 한다(SHALL). 404 응답은 "robots.txt 없음 = 모두 허용"으로 해석해야 한다(SHALL). 실패 상태 역시 24시간 TTL과 함께 캐시하여 연속 재시도로 인한 폭주를 방지해야 한다(SHALL).

#### Scenario: 네트워크 오류 시 fail-open
- **WHEN** 호스트의 robots.txt fetch가 타임아웃되거나 네트워크 오류로 실패할 때
- **THEN** 해당 호스트의 모든 링크는 RobotsFilter를 통과한다

#### Scenario: 404 응답은 규칙 없음으로 해석
- **WHEN** robots.txt가 404로 응답할 때
- **THEN** 해당 호스트에는 제한이 없는 것으로 간주하여 모든 링크가 통과한다

#### Scenario: 5xx 응답 시 fail-open 상태 캐시
- **WHEN** robots.txt가 5xx로 응답할 때
- **THEN** 해당 호스트는 fail-open 상태로 캐시되며, TTL 이내에는 재시도하지 않는다

#### Scenario: 404·5xx 외 비-2xx 응답은 "규칙 없음"으로 해석한다
- **WHEN** robots.txt가 404·5xx 이외의 비-2xx 응답(예: 인증 요구 401/403, 비정상 3xx, 429)을 반환할 때
- **THEN** 해당 호스트는 "규칙 없음"으로 간주되어 모든 링크가 통과하되, fail-open 상태가 아닌 "빈 규칙"으로 캐시된다(실패 상태와 다르게 정책 확대 해석을 하지 않음)

---

### Requirement: RobotsFilter는 Crawl-delay를 호스트 bucket에 반영한다
RobotsFilter는 robots.txt에서 `Crawl-delay: N` (초) 지시어를 파싱해야 하며(SHALL), 파싱에 성공한 경우 `scheduler-host-token-bucket` capability의 **호스트 rate/burst 설정 동작**을 호출하여 해당 호스트의 rate를 `1/N` req/sec, burst `1`로 갱신해야 한다(SHALL). Crawl-delay가 없거나 파싱에 실패한 경우 호스트 rate/burst 설정 동작을 호출하지 않으며 scheduler의 기본 rate가 유지되어야 한다(SHALL).

#### Scenario: Crawl-delay 파싱 및 호스트 rate 갱신
- **WHEN** robots.txt에 `Crawl-delay: 5`가 포함될 때
- **THEN** RobotsFilter는 해당 호스트에 대해 scheduler의 호스트 rate/burst 설정 동작을 호출하여 rate가 초당 0.2 requests, burst가 1로 갱신된다

#### Scenario: Crawl-delay 미지정 시 기본 rate 유지
- **WHEN** robots.txt에 Crawl-delay가 명시되지 않을 때
- **THEN** 호스트 rate/burst 설정 동작은 호출되지 않고 scheduler의 기본 rate가 유지된다

#### Scenario: 파싱 불가능한 Crawl-delay 무시
- **WHEN** Crawl-delay 값이 정수/실수로 파싱되지 않을 때
- **THEN** 해당 값은 무시되고 호스트 rate/burst 설정 동작은 호출되지 않는다

#### Scenario: 캐시 TTL 내 중복 호출 방지
- **WHEN** 같은 호스트에 대해 24시간 캐시 TTL 이내에 다수 링크가 필터링될 때
- **THEN** 호스트 rate/burst 설정 동작은 캐시 갱신 시점(최초 fetch 또는 TTL 만료 재fetch)에만 호출된다

---

### Requirement: Pioneer 부트스트랩은 RobotsFilter에 HostRateLimiter를 wire한다

Pioneer 워커의 엔트리포인트(`apps/api/cmd/bot`)는 FilterChain의 RobotsFilter를 생성할 때, 동일 워커 인스턴스의 scheduler가 사용하는 `*scheduler.HostRateLimiter`와 **같은 인스턴스**를 `HostRateSetter` 인자로 전달해야 한다(SHALL). Pioneer 부트스트랩은 RobotsFilter의 `HostRateSetter` 인자로 `nil`이나 별개 인스턴스를 전달해서는 안 된다(SHALL NOT).

본 Requirement는 기존 Requirement `RobotsFilter는 Crawl-delay를 호스트 bucket에 반영한다`의 Scenario "Crawl-delay 파싱 및 호스트 rate 갱신"·"캐시 TTL 내 중복 호출 방지"가 production pioneer 워커에서 enforce되도록 보장하는 wiring 계약이다. 기존 Requirement의 SHALL 본문과 4개 Scenarios의 의미는 변경하지 않는다.

#### Scenario: Pioneer 워커 부트스트랩이 RobotsFilter와 scheduler에 동일한 host rate limiter 인스턴스를 전파한다

- **WHEN** Pioneer 워커가 `runPioneerConsumer`로 부팅되어 PioneerConsumer를 조립할 때
- **THEN** FilterChain 안의 RobotsFilter는 scheduler가 dequeue 시점에 host bucket을 조회하는 인스턴스와 동일한 `*scheduler.HostRateLimiter`를 `HostRateSetter`로 보유한다. RobotsFilter가 새 호스트의 robots.txt 파싱 후 `SetHostRate`를 호출하면 그 변경이 같은 워커의 다음 dequeue부터 호스트 token bucket에 즉시 반영된다.

#### Scenario: Pioneer 워커 부트스트랩이 RobotsFilter에 nil을 전달하지 않는다

- **WHEN** Pioneer 워커가 production 부트스트랩 경로로 PioneerConsumer를 조립할 때
- **THEN** FilterChain 안의 RobotsFilter는 nil이 아닌 `HostRateSetter`를 보유한다. RobotsFilter가 `Crawl-delay: N`을 정상 파싱하면 `SetHostRate(host, 1/N, 1)`가 실제로 호출되어 기존 Requirement `RobotsFilter는 Crawl-delay를 호스트 bucket에 반영한다`의 Scenario "Crawl-delay 파싱 및 호스트 rate 갱신"이 production에서 관찰된다.

---

### Requirement: Harvester는 Pin 생성 시 primary 이미지를 object storage에 캐시한다
시스템은 Harvester가 새 Pin을 생성할 때, 해당 페이지에서 추출한 primary 이미지 후보가 있으면 그 이미지를 우리 object storage에 저장하고, 저장 결과에 해당하는 참조 값을 Pin의 **대표 이미지 참조 속성**에 기록해야 한다(SHALL). 이미지 캐싱의 성공/실패는 Pin 생성 자체의 성공/실패에 영향을 주지 않아야 한다(SHALL NOT block Pin creation).

본 requirement는 기존 `bot` capability의 "Harvester는 미디어 파일을 스토리지에 다운로드하여 저장한다" requirement와는 **별개의 데이터 흐름**이며 서로의 실패 정책에 영향을 주지 않는다. 본문 미디어(item의 media 본체) 다운로드 실패는 기존 정책에 따라 item skip을 야기할 수 있으나, primary 이미지 캐시 실패는 본 requirement의 fallback 경로로만 처리되고 Pin 생성을 막지 않는다. Pin의 "대표 이미지 참조 속성"이 저장 스키마의 어느 컬럼에 매핑되는지는 구현 관심사이며 design 문서에서 확정한다(본 change 기준으로는 단일 속성을 사용한다).

본 capability에서 이미지 캐시 동작의 **외부 관찰 가능 행위**는 다음과 같다:
- **성공**: Pin의 대표 이미지 참조 속성에 object storage 상의 참조가 기록된다.
- **실패**: 다운로드 실패, 업로드 실패, 크기 초과 중 어느 것이든 **구분 없이** 동일하게 처리되어, 채택된 원본 후보 URL이 대표 이미지 참조 속성의 값으로 기록된다.
- **후보 없음**: 대표 이미지 참조 속성은 비어 있는(기록되지 않은) 상태로 남는다.

#### Scenario: 이미지 캐시 성공 시 storage 참조를 Pin에 기록
- **WHEN** Harvester가 페이지에서 primary 이미지 후보 URL을 찾고, 다운로드와 object storage 업로드가 모두 성공할 때
- **THEN** 시스템은 Pin의 대표 이미지 참조 속성에 object storage 참조를 기록한다

#### Scenario: 이미지 후보가 존재하지 않을 때
- **WHEN** Harvester가 추출 우선순위에 따른 모든 후보를 시도했지만 유효한 이미지 URL을 찾지 못할 때
- **THEN** 시스템은 Pin의 대표 이미지 참조 속성을 비워 두고 Pin은 정상 생성한다

#### Scenario: 이미지 캐시 실패해도 Pin 생성은 계속된다
- **WHEN** 이미지 후보는 찾았지만 다운로드·업로드·크기 초과 중 어느 하나로 캐시가 실패할 때
- **THEN** 시스템은 Pin 생성을 실패시키지 않고, Pin의 대표 이미지 참조 속성에 원본 후보 URL을 기록하며, 실패 사유는 로그로 관찰 가능하다

---

### Requirement: 이미지 추출은 og:image → twitter:image → article 내 의미 있는 img → JSON-LD image 우선순위를 따른다
시스템은 Pin의 primary 이미지 후보를 추출할 때 다음 4단계를 위에서 아래로 시도하고, 첫 번째로 유효한 후보를 채택해야 한다(SHALL): (1) `<meta property="og:image">`, (2) `<meta name="twitter:image">` 또는 `<meta property="twitter:image">`, (3) `<article>` 또는 `<main>` 내부의 의미 있는 `<img>`, (4) `<script type="application/ld+json">` 안 schema.org 객체의 `image` 필드. "유효"는 (a) URL이 절대 URL로 resolve 가능, (b) http 또는 https 스킴, (c) `data:` URI가 아님, (d) 명백한 추적 픽셀(1×1 이미지 등)로 의심되지 않음을 모두 만족해야 한다(SHALL).

동일 우선순위 단계에서 여러 후보가 발견될 때(예: 여러 `<script type="application/ld+json">` 블록, 또는 JSON-LD `image` 필드가 배열/객체 형태), 시스템은 **문서 내 등장 순서(DOM 순서)** 기준 첫 번째 유효 후보를 채택해야 한다(SHALL).

#### Scenario: og:image가 존재하면 1순위로 채택
- **WHEN** 페이지에 `<meta property="og:image" content="https://example.com/cover.jpg">` 가 있고, twitter:image와 article img도 함께 존재할 때
- **THEN** 시스템은 og:image의 URL을 채택한다

#### Scenario: og:image가 없으면 twitter:image 채택
- **WHEN** 페이지에 og:image는 없고 `<meta name="twitter:image" content="https://example.com/tw.jpg">` 가 있을 때
- **THEN** 시스템은 twitter:image의 URL을 채택한다

"article 내 의미 있는 `<img>`"는 다음 기준 중 **어느 하나라도** 만족하는 `<article>`/`<main>` 내부 `<img>` 요소를 의미한다(SHALL): (i) `width` 속성과 `height` 속성이 **둘 다** 100 이상, 또는 (ii) `alt` 속성이 비어 있지 않음. 위 기준을 만족하지 않는 `<img>`(예: width/height 미지정이거나 하나만 지정된 작은 img, alt가 공백)는 본 우선순위 단계에서 후보가 아니다(SHALL NOT).

#### Scenario: og/twitter가 모두 없으면 article 내 의미 있는 img 채택
- **WHEN** og:image와 twitter:image가 모두 없고, `<article>` 또는 `<main>` 내부에 `width="600" height="400"` 처럼 둘 다 100 이상을 만족하거나 `alt="상품 사진"` 처럼 비어있지 않은 alt를 갖는 `<img>`가 있을 때
- **THEN** 시스템은 해당 article/main 내부의 DOM 순서상 첫 번째 그러한 `<img>` 의 src를 채택한다

#### Scenario: 크기 기준·alt 기준 어느 쪽도 충족 못하는 img는 article 후보가 아니다
- **WHEN** `<article>` 내부에 `<img src="icon.png">` 처럼 width/height 속성이 없고 alt도 비어 있는 `<img>`만 있을 때
- **THEN** 시스템은 해당 img를 후보에서 제외하고 다음 우선순위 단계(JSON-LD)로 진행한다

#### Scenario: 위 셋이 모두 없으면 JSON-LD image 채택
- **WHEN** og:image, twitter:image, article 내 유효 img가 모두 없고, `<script type="application/ld+json">` 안에 `"image": "https://example.com/ld.jpg"` 또는 `"image": ["https://example.com/ld.jpg", ...]` 또는 `"image": {"url": "https://example.com/ld.jpg"}` 가 있을 때
- **THEN** 시스템은 DOM 순서상 첫 번째 JSON-LD 블록의 첫 번째 유효 image URL을 채택한다(배열이면 첫 요소, 객체면 `url` 필드)

#### Scenario: 상대 URL 후보는 절대 URL로 해석되어 채택된다
- **WHEN** 채택된 후보 속성 값이 `/static/cover.jpg` 같은 상대 경로일 때
- **THEN** 시스템은 페이지 URL을 base로 하여 절대 URL로 해석한 값을 후보로 사용한다

#### Scenario: data: URI는 후보에서 제외된다
- **WHEN** og:image의 값이 `data:image/png;base64,...` 일 때
- **THEN** 시스템은 해당 후보를 건너뛰고 다음 우선순위 단계로 진행한다

#### Scenario: 추적 픽셀로 의심되는 후보는 제외된다
- **WHEN** 후보 URL이 1×1 추적 픽셀로 의심되는 특성(예: 파일명이 추적 픽셀 관례적 패턴을 포함)을 가질 때
- **THEN** 시스템은 해당 후보를 건너뛰고 다음 후보 또는 다음 우선순위 단계로 진행한다

---

### Requirement: 이미지 캐시 객체는 후보 URL에서 파생된 안정적이고 충돌 회피된 키로 저장된다
시스템은 캐시할 이미지를 object storage에 저장할 때, 후보 URL과 저장 시점에서 결정적으로 파생되는 키로 저장해야 한다(SHALL). 키 구성은 다음 외부 관찰 가능 조건을 만족해야 한다:
- 서로 다른 후보 URL은 서로 다른 키로 저장된다(SHALL).
- 같은 후보 URL을 서로 다른 시점에 캐시하면 서로 다른 키로 저장되어, 이전 객체가 덮어써지지 않는다(SHALL NOT overwrite).
- 이미지 캐시 저장 네임스페이스는 본문 미디어(item의 media 본체) 저장 네임스페이스와 **분리**되어, 두 경로의 모니터링/lifecycle 정책을 독립적으로 운용할 수 있다(SHALL).
- 저장 키는 응답 Content-Type에서 파생된 확장자를 포함해야 하며(SHALL), Content-Type이 없거나 매핑되지 않을 때의 fallback 또한 결정적이어야 한다(SHALL).

구체 해시 알고리즘, 타임스탬프 해상도, Content-Type ↔ 확장자 매핑 테이블, 네임스페이스 이름 같은 **키 구성의 내부 알고리즘**은 design 문서에서 확정한다.

단, 스킴과 호스트의 **대소문자 차이만**(RFC 3986상 case-insensitive한 컴포넌트)은 서로 다른 후보로 취급하지 않는다 — 예: `HTTP://Example.com/x` 와 `http://example.com/x` 는 동일 후보로 간주되어 동일 키 공간에 저장된다.

#### Scenario: 서로 다른 후보 URL은 서로 다른 키로 저장된다
- **WHEN** 두 후보 URL이 정규형(스킴·호스트는 대소문자 동일시, path·query는 문자 그대로) 기준으로 스킴/호스트/경로/쿼리 중 어느 하나라도 다를 때
- **THEN** 두 객체의 저장 키는 서로 다르다

#### Scenario: 스킴/호스트 대소문자 차이만 있는 두 후보는 동일 후보로 취급된다
- **WHEN** 두 후보 URL이 `HTTPS://Example.com/a.jpg` 와 `https://example.com/a.jpg` 처럼 스킴과 호스트의 대소문자만 다르고 path·query가 동일할 때
- **THEN** 두 후보는 동일 후보로 간주되어 저장 키 파생의 관점에서 동일한 키 공간에 매핑된다

#### Scenario: 같은 후보 URL을 다른 시점에 재캐시하면 별도 객체로 저장된다
- **WHEN** 동일 후보 URL을 서로 다른 시점에 두 번 캐시할 때
- **THEN** 두 번째 업로드는 첫 번째 객체를 덮어쓰지 않고 별도 키로 저장된다

#### Scenario: 응답 Content-Type에서 확장자가 파생된다
- **WHEN** 다운로드 응답의 Content-Type이 이미지 타입을 명시할 때
- **THEN** 저장 키에는 해당 Content-Type에 대응되는 확장자가 포함된다

#### Scenario: Content-Type 확장자 파생이 실패하면 결정적 fallback 확장자가 사용된다
- **WHEN** 다운로드 응답의 Content-Type이 알려진 이미지 타입 매핑에 없을 때
- **THEN** 저장 키는 원본 URL의 확장자 또는 사전에 정의된 기본 확장자를 결정적 규칙으로 사용한다

#### Scenario: 이미지 캐시 네임스페이스가 본문 미디어 네임스페이스와 분리된다
- **WHEN** 시스템이 이미지 캐시 객체와 본문 미디어 객체를 모두 저장할 때
- **THEN** 두 저장 위치는 분리된 네임스페이스를 가져 서로 prefix 충돌이나 lifecycle 교차가 없다

---

### Requirement: 이미지 캐시 실패는 단일 fallback 경로로 처리된다
시스템은 후보 URL은 찾았으나 (a) 다운로드 실패, (b) 업로드 실패, (c) 응답 Content-Length 혹은 read 누적이 구현 임계치를 초과 중 **어느 것이든** 발생하면, **구분 없이 동일하게** Pin의 대표 이미지 참조 속성에 채택된 원본 후보 URL을 그대로 기록해야 한다(SHALL). Pin 생성은 성공으로 처리되어야 하며, 부분적으로 다운로드된 바이트는 object storage에 업로드되지 않아야 한다(SHALL NOT upload partial bytes). 업로드 도중 실패로 인해 부분 객체(예: 중단된 멀티파트 업로드 또는 partial commit)가 남을 수 있는 경우, **해당 객체의 정리 책임은 본 capability 외부**(storage lifecycle 또는 후속 GC change)에 위임된다. 실패 사유는 로그로 관찰 가능해야 한다(SHALL).

#### Scenario: 다운로드 실패 시 원본 URL fallback
- **WHEN** 후보 URL이 다운로드 단계에서 HTTP 403/404/타임아웃 등으로 실패할 때
- **THEN** 시스템은 Pin의 대표 이미지 참조 속성에 채택된 원본 후보 URL을 기록하고 Pin 생성을 성공시킨다

#### Scenario: 업로드 실패 시 원본 URL fallback
- **WHEN** 다운로드는 성공했으나 object storage 업로드가 실패할 때
- **THEN** 시스템은 Pin의 대표 이미지 참조 속성에 원본 후보 URL을 기록하고 Pin 생성을 성공시킨다

#### Scenario: 다운로드 크기가 임계치를 초과하면 fallback 및 부분 데이터 버림
- **WHEN** 응답 Content-Length 또는 실제 다운로드 바이트가 시스템 임계치를 초과할 때
- **THEN** 시스템은 다운로드를 즉시 중단하고, 부분 데이터를 storage에 업로드하지 않으며, 원본 URL을 대표 이미지 참조 속성에 기록한다

#### Scenario: 실패는 관찰 가능하다
- **WHEN** 이미지 캐시가 fallback 경로로 처리될 때
- **THEN** 시스템은 실패 사유(다운로드/업로드/크기초과)를 식별 가능한 로그로 기록한다

### Requirement: 캐시된 primary 이미지 객체는 설정 가능한 TTL 후 만료 대상이 된다
시스템은 이미지 캐시 네임스페이스에 저장된 primary 이미지 객체에 대해 **연령 기반 TTL**을 capability 내 계약으로 정의해야 한다(SHALL). 각 캐시 객체는 자신의 작성/최종 쓰기 시점으로부터 TTL이 경과한 시점부터 시스템에 의해 **제거 대상(eligible for removal)** 상태가 되어야 하며(SHALL), TTL 미경과 시점에서는 **연령 기반 만료 메커니즘에 의한** 제거 대상이 아니어야 한다(SHALL NOT expire before TTL). 이 금지는 만료 메커니즘에 한정되며, 요구 `미참조가 된 이미지 캐시 객체는 처리 경로에서 정리된다`에 따른 미참조 객체의 삭제는 TTL 경과 여부와 무관하게 허용된다. TTL 값은 운영자가 설정 가능해야 한다(SHALL be configurable).

본 requirement는 **primary 이미지 캐시 네임스페이스 한정**이다. 본문 미디어(item의 media 본체) 저장 네임스페이스의 만료 정책은 본 requirement의 범위가 아니다. 제거 대상 여부의 판정 근거와 실제 제거를 수행하는 메커니즘, TTL의 기본값과 설정 키 이름은 **내부 구현 세부**이며 design 문서에서 확정한다.

캐시 객체의 만료는 Pin 생성 경로와 **비동기**이다. 만료 처리의 성공/실패는 Pin 생성의 성공/실패에 영향을 주지 않아야 한다(SHALL NOT block Pin creation). 구체적으로, 기존의 이미지 캐시 실패 fallback 동작(다운로드/업로드/용량 초과 시 원본 후보 URL로 기록), 후보가 없을 때 공란으로 남는 동작, 캐시 성공 시점의 관찰 가능 결과는 TTL 설정 여부 및 만료 처리 상태와 **무관하게 보전**되어야 한다(SHALL preserve existing cache-path observable behavior). 만료로 인한 참조 해소 실패 가능성은 Pin 이후 조회 시점의 사후 현상이며, 그 참조의 해소 결과(예: 404)는 본 capability의 실패로 간주하지 않는다. 이 경우 Pin 자체는 유효하며, 소비자 측 UX가 참조 해소 실패를 허용해야 한다.

동일 후보 URL의 재캐시가 별도 객체로 저장된다는 기존 Requirement("이미지 캐시 객체는 후보 URL에서 파생된 안정적이고 충돌 회피된 키로 저장된다")는 유지된다. 따라서 만료는 **객체 단위**로 평가되며, 같은 후보 URL의 여러 객체가 각각 자신의 작성/최종 쓰기 시점 기준 TTL에 따라 독립적으로 제거 대상이 된다.

#### Scenario: TTL 미경과 객체는 만료에 의한 제거 대상이 아니다
- **WHEN** 이미지 캐시 객체의 작성/최종 쓰기 시점으로부터 TTL이 아직 경과하지 않은 시점에 시스템이 만료 판정을 수행할 때
- **THEN** 해당 객체는 만료에 의한 제거 대상이 아니며 만료 메커니즘에 의해 storage에서 제거되지 않는다 (미참조 정리 요구에 따른 삭제는 본 판정과 무관하게 허용된다)

#### Scenario: TTL 경과 객체는 제거 대상이 된다
- **WHEN** 이미지 캐시 객체의 작성/최종 쓰기 시점으로부터 TTL이 경과한 이후 시점에 시스템이 만료 판정을 수행할 때
- **THEN** 해당 객체는 제거 대상으로 분류되고, 시스템의 제거 메커니즘에 의해 storage에서 삭제된다

#### Scenario: TTL 값은 설정 가능하다
- **WHEN** 운영자가 TTL 설정 값을 기본값과 다른 값으로 지정할 때
- **THEN** 모든 이후 캐시 객체의 만료 판정은 지정된 TTL 값을 기준으로 수행된다

#### Scenario: 만료 처리는 Pin 생성을 막지 않는다
- **WHEN** 만료 처리가 동시에 실행 중이거나 실패한 상태에서 Harvester가 새 Pin을 생성할 때
- **THEN** Pin 생성은 만료 처리의 상태와 무관하게 기존 이미지 캐시 Requirement의 성공/실패 기준에 따라 독립적으로 처리된다

#### Scenario: 만료된 객체 참조는 capability 실패가 아니다
- **WHEN** Pin의 대표 이미지 참조 속성이 TTL 경과 이후 시점에 소비자 조회 시 해소되지 않을 때
- **THEN** 본 capability의 이미지 캐시 Requirement는 여전히 "Pin 생성 시점"의 성공 기준으로 판정되며, 사후 해소 실패는 본 capability의 실패로 집계되지 않는다

#### Scenario: 동일 후보 URL의 두 객체는 각자 TTL을 가진다
- **WHEN** 같은 후보 URL이 시점 T1과 T2(T2 > T1)에 각각 별도 객체로 캐시되고 T1의 객체만 TTL이 경과한 시점에 판정할 때
- **THEN** T1 시점에 저장된 객체만 제거 대상이 되고 T2 시점에 저장된 객체는 제거 대상이 아니다

#### Scenario: 만료는 primary 이미지 캐시 네임스페이스에만 적용된다
- **WHEN** 시스템이 본 capability의 TTL 만료 판정을 수행할 때
- **THEN** 본문 미디어(item의 media 본체) 저장 네임스페이스의 객체는 판정 대상에 포함되지 않는다

### Requirement: 미참조가 된 이미지 캐시 객체는 처리 경로에서 정리된다

시스템은 Pin 문서 처리 과정에서 자사 저장소의 이미지 캐시 객체가 처리 중인 해당 Pin에서 더 이상 참조되지 않게 되는 두 경우에 대해 해당 객체를 삭제해야 한다(SHALL). 캐시 키 특성상 드물게 다른 Pin이 같은 객체를 참조하고 있을 수 있으며, 그 경우 남는 참조 끊김(dangling reference)은 본 capability의 실패가 아니다:

1. **저장 실패 보상**: 새 캐시 객체 저장에 성공한 뒤 Pin의 영속화(신규 생성 또는 갱신)가 실패하면, 그 시도에서 새로 저장된 캐시 객체를 삭제해야 한다(SHALL). Pin 처리의 실패 결과 자체는 변경되지 않아야 한다(SHALL NOT change failure semantics).
2. **교체 정리**: Pin의 영속화가 성공했고 그 결과 대표 이미지 참조가 이전과 **다른 값**으로 교체되었으며 이전 값이 자사 저장소의 캐시 객체를 가리키던 경우, 그 이전 객체를 삭제해야 한다(SHALL). 새 값이 자사 캐시 객체인지, 원본 URL fallback인지, 참조 부재인지와 무관하게 적용된다.

삭제는 비차단(best-effort)이어야 한다(SHALL): 삭제의 성공/실패는 Pin 처리의 성공/실패, 생성/갱신 판정, 반환되는 Pin 식별자에 영향을 주지 않아야 하며(SHALL NOT), 삭제 실패는 대상과 사유가 식별 가능한 로그로 관찰 가능해야 한다(SHALL). 삭제가 실패하거나 누락된 객체는 기존 요구 `캐시된 primary 이미지 객체는 설정 가능한 TTL 후 만료 대상이 된다`의 TTL 만료가 최종 방어선으로 회수한다.

삭제 대상은 **이미지 캐시 네임스페이스에 속하는 객체로 한정**된다(SHALL). 자사 저장소를 가리키지 않는 이전 참조(원본 후보 URL fallback으로 기록된 값)는 삭제 대상이 아니며(SHALL NOT attempt deletion of external URLs), 자사 저장소라도 이미지 캐시 네임스페이스 밖의 객체(사용자 업로드 미디어, 본문 미디어 등)는 어떤 경우에도 삭제하지 않는다(SHALL NOT). 참조 값이 교체되지 않은 경우(이전 값과 새 값이 동일) 삭제하지 않는다(SHALL NOT).

기존 요구 `이미지 캐시 객체는 후보 URL에서 파생된 안정적이고 충돌 회피된 키로 저장된다`(no-overwrite)와 `이미지 캐시 실패는 단일 fallback 경로로 처리된다`의 행위는 변경되지 않는다. 기존 fallback 요구가 외부에 위임한 정리 책임은 업로드 도중 실패로 남는 부분 객체에 한정되며, 본 요구는 **완결 저장된 후 미참조가 된 객체**를 다룬다. 요구 `캐시된 primary 이미지 객체는 설정 가능한 TTL 후 만료 대상이 된다`의 만료 금지는 연령 기반 만료 메커니즘에 한정되며, 본 요구에 따른 미참조 객체 삭제에는 적용되지 않는다.

#### Scenario: Pin 영속화 실패 시 새 캐시 객체가 보상 삭제된다

- **WHEN** 새 캐시 객체 저장에 성공한 뒤 해당 Pin의 영속화가 실패할 때
- **THEN** 시스템은 그 시도에서 새로 저장된 캐시 객체를 삭제하고, Pin 처리는 기존과 동일하게 실패로 반환된다

#### Scenario: 캐시 저장 자체가 실패(fallback)한 처리의 영속화 실패에는 보상 삭제가 없다

- **WHEN** 캐시 저장이 실패하여 원본 후보 URL fallback으로 진행하던 처리에서 Pin의 영속화가 실패할 때
- **THEN** 시스템은 어떤 삭제도 시도하지 않는다 (그 시도에서 저장된 객체가 없으므로)

#### Scenario: 재수집으로 새 캐시 객체로 교체되면 이전 객체가 삭제된다

- **WHEN** 재수집으로 Pin의 대표 이미지 참조가 이전의 자사 캐시 객체에서 새로 저장된 자사 캐시 객체로 교체될 때
- **THEN** 시스템은 이전 캐시 객체를 삭제하고, 새 객체와 Pin 처리 결과는 영향받지 않는다

#### Scenario: 재수집이 원본 URL fallback으로 귀결되어도 이전 객체가 삭제된다

- **WHEN** 재수집에서 캐시 저장이 실패하여 대표 이미지 참조가 이전의 자사 캐시 객체에서 원본 후보 URL로 교체될 때
- **THEN** 시스템은 이전 캐시 객체를 삭제한다

#### Scenario: 재수집에서 대표 이미지 후보가 사라져 참조가 비워져도 이전 객체가 삭제된다

- **WHEN** 재수집에서 대표 이미지 후보가 없거나 캐시가 비활성이어서 대표 이미지 참조가 이전의 자사 캐시 객체에서 참조 부재로 교체될 때
- **THEN** 시스템은 이전 캐시 객체를 삭제한다

#### Scenario: 이전 참조가 외부 URL이면 삭제를 시도하지 않는다

- **WHEN** 재수집으로 대표 이미지 참조가 교체되었으나 이전 값이 자사 저장소가 아닌 외부 URL일 때
- **THEN** 시스템은 어떤 삭제도 시도하지 않는다

#### Scenario: 자사 저장소라도 캐시 네임스페이스 밖의 객체는 삭제하지 않는다

- **WHEN** 재수집으로 대표 이미지 참조가 교체되었으나 이전 값이 자사 저장소의 이미지 캐시 네임스페이스 밖 객체(예: 사용자 업로드 미디어)를 가리킬 때
- **THEN** 시스템은 어떤 삭제도 시도하지 않는다

#### Scenario: 참조가 교체되지 않으면 삭제하지 않는다

- **WHEN** 재수집 후 대표 이미지 참조의 이전 값과 새 값이 동일할 때
- **THEN** 시스템은 어떤 삭제도 시도하지 않는다

#### Scenario: 신규 Pin 생성에는 교체 정리가 없다

- **WHEN** 처음 수집되는 canonical URL로 새 Pin이 생성될 때 (이전 참조가 존재하지 않음)
- **THEN** 시스템은 어떤 삭제도 시도하지 않는다

#### Scenario: 삭제 실패는 Pin 처리 결과에 영향을 주지 않는다

- **WHEN** 교체 정리 또는 보상 삭제의 삭제 호출이 실패할 때
- **THEN** Pin 처리의 성공/실패·생성/갱신 판정·반환 식별자는 삭제를 수행하지 않았을 때와 동일하고, 삭제 실패는 대상과 사유가 식별 가능한 로그로 기록되며, 해당 객체는 TTL 만료로 회수된다

### Requirement: 스냅샷 사용 불가 시 HTTP fetch로 폴백한다

ObjectStorage 조회가 성공하지 못하는 모든 경우(키 없음, 네트워크/권한/내부 에러 등 일체 — TTL 만료는 lifecycle 삭제에 의해 "키 없음"으로 수렴하며 독립 관측 범주가 아니다)를 Harvester는 단일 "사용 불가(miss)"로 취급하여 동일 노드 URL에 대해 HTTP fetch로 폴백해야 한다(SHALL). 실패 유형에 따라 fetch 동작이 달라져서는 안 된다(MUST NOT). 폴백된 HTTP 응답을 ObjectStorage에 재저장할지 여부는 본 요구사항이 정의하지 않으며, 저장 책임은 Pioneer 쓰기 경로에 위임한다.

ObjectStorage 실패 유형 구분은 **로그/메트릭 레벨에서만** 수행되어야 하며(운영 관찰·알람 임계치 산정용), fetch 의사결정(폴백 여부)에는 영향을 주지 않는다(SHALL). 이는 동작(behavior)이 아니라 관측(observability)의 영역이다.

#### Scenario: 스냅샷 miss 시 HTTP 폴백
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청했으나 ObjectStorage에 해당 스냅샷이 존재하지 않을 때(신규 미저장, lifecycle rule에 의한 TTL 만료 삭제, 과거 UTC 일자 키 어긋남 등이 모두 이 단일 케이스로 수렴)
- **THEN** 동일 URL에 대해 HTTP fetch를 수행하여 본문을 획득한 뒤 파싱 파이프라인을 진행한다

#### Scenario: ObjectStorage 에러 시 HTTP 폴백
- **WHEN** Harvester의 ObjectStorage 조회가 네트워크/권한/내부 에러로 실패할 때
- **THEN** Harvester는 즉시 실패로 처리하지 않고 동일 URL에 대해 HTTP fetch로 폴백한다

#### Scenario: 실패 유형은 로그로만 구분된다
- **WHEN** ObjectStorage 조회가 실패할 때
- **THEN** 운영자가 실패 원인을 로그·메트릭을 통해 판별할 수 있도록 관측 데이터가 남지만, fetch 동작은 모든 케이스에서 동일하게 HTTP fallback으로 수렴한다(실패 유형이 fetch 분기에 영향을 주지 않는다). 관측 라벨의 구체적 문자열 집합은 본 spec의 행위 계약 대상이 아니며 운영 설정에서 관리한다

---

### Requirement: ObjectStorage와 HTTP 모두 실패 시 노드 처리 실패로 집계한다

ObjectStorage 경로와 HTTP 폴백 경로가 모두 본문을 반환하지 못하면, Harvester는 해당 노드 처리를 실패로 분류하고 **Harvester 워커의 실행 통계(in-memory, scheduler의 DB 컬럼과 구분되는 별개 집계)** 의 fetch 실패 카운터가 1 증가하도록 해야 한다(SHALL). 이 실패는 다른 노드의 처리를 중단시키지 않아야 하며(SHALL), 단일 노드 실패가 Harvester 실행 전체를 중단시켜서는 안 된다(MUST NOT). 집계 카운터의 내부 식별자 이름은 본 spec의 행위 계약 대상이 아니며 구현 문서에서 정의한다. `harvester_frontier.harvest_error_count` DB 컬럼 증가는 `harvester-scheduler-consumer` capability의 `RecordHarvestError` 경로가 담당하며, 본 Requirement는 해당 DB 경로에 추가 증가를 요구하지 않는다.

#### Scenario: 이중 실패 시 실행 통계 카운터 증가
- **WHEN** Harvester가 노드 URL에 대해 fetch를 요청했고 ObjectStorage 조회와 HTTP 폴백이 모두 본문을 반환하지 못할 때
- **THEN** 해당 노드의 파싱은 수행되지 않고 Harvester 워커 실행 통계의 fetch 실패 카운터가 정확히 1 증가한다

#### Scenario: 노드 단위 실패 격리
- **WHEN** 특정 노드의 fetch가 이중 실패로 종료될 때
- **THEN** Harvester는 다음 노드의 처리를 계속 진행하며 단일 노드의 실패가 전체 실행을 중단시키지 않는다

#### Scenario: fetch 출처 및 실패 종류의 관측성
- **WHEN** Harvester가 fetch를 수행할 때
- **THEN** 각 호출의 fetch 출처(스냅샷/HTTP) 및 실패 시 ObjectStorage 에러 종류가 로그/메트릭으로 식별 가능하다(운영 관찰용이며 fetch 행위에는 영향을 주지 않는다)

