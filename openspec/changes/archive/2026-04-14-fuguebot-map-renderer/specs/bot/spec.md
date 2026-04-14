## Requirements

### Requirement: URL 패턴으로 페이지 타입을 분류한다 [MODIFIED]
시스템은 발견한 URL을 키워드 매칭, 경로 패턴, 쿼리 파라미터 분석을 통해 타입(list, detail, skip)으로 분류해야 한다.

#### Scenario: list 페이지 분류 (trending/popular 키워드)
- **WHEN** URL 경로가 'trending', 'popular', 'hot', 'featured', 'recent', 'explore' 키워드를 포함할 때
- **THEN** 노드 타입이 'list'로 설정된다

#### Scenario: list 페이지 분류 (gallery 키워드)
- **WHEN** URL 경로가 'gallery', 'collection', 'album', 'showcase' 키워드를 포함할 때
- **THEN** 노드 타입이 'list'로 설정된다

#### Scenario: list 페이지 분류 (category 키워드)
- **WHEN** URL 경로가 'category', 'tag', 'genre', 'style', 'contest', 'event' 키워드를 포함할 때
- **THEN** 노드 타입이 'list'로 설정된다

#### Scenario: detail 페이지 분류 (경로 패턴)
- **WHEN** URL 경로에 4자리 이상 숫자 ID 패턴이 있고 고우선순위 키워드가 없을 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: detail 페이지 분류 (쿼리 파라미터)
- **WHEN** URL 쿼리 파라미터에 id, illust_id 등 명시적 ID 키의 숫자 값이 포함되어 있을 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: detail 페이지 분류 (콘텐츠 단수형 경로)
- **WHEN** URL 경로에 '/artworks/', '/photos/', '/works/', '/illust/' 등 단수형 콘텐츠 세그먼트가 포함될 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: 불필요한 URL 제외
- **WHEN** URL이 'ad', 'popup', 'login', 'signup', 'cart', 'checkout' 키워드를 포함할 때
- **THEN** 해당 URL은 그래프에 추가되지 않는다

#### Scenario: 기본 분류
- **WHEN** URL이 어떤 패턴에도 매칭되지 않을 때
- **THEN** 노드 타입이 'list'로 설정된다

---

### Requirement: 그래프 노드와 엣지를 관리한다 [MODIFIED]

#### Scenario: 사이트의 모든 list 페이지 조회
- **WHEN** 특정 사이트의 list 타입 노드들을 조회할 때
- **THEN** 시스템은 해당하는 모든 노드를 빠르게 반환한다
