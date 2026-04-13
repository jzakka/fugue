## MODIFIED Requirements

### Requirement: URL 패턴으로 페이지 타입을 분류한다
시스템은 발견한 URL을 키워드 매칭을 통해 타입(listing, gallery, detail, category)으로 분류해야 한다(SHALL). classifyURL은 순수한 타입 분류만 수행하며, skip 판정은 FilterChain의 PathPatternFilter가 담당한다(SHALL).

#### Scenario: listing 페이지 분류
- **WHEN** URL이 'trending', 'popular', 'hot', 'featured', 'recent' 키워드를 포함할 때
- **THEN** 노드 타입이 'listing'으로 설정된다

#### Scenario: detail 페이지 분류
- **WHEN** URL이 숫자 ID 패턴을 가지고 고우선순위 키워드가 없을 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: gallery 페이지 분류
- **WHEN** URL이 'gallery', 'collection', 'album', 'showcase' 키워드를 포함할 때
- **THEN** 노드 타입이 'gallery'로 설정된다

#### Scenario: category 페이지 분류
- **WHEN** URL이 'category', 'tag', 'genre', 'style', 'contest', 'event' 키워드를 포함할 때
- **THEN** 노드 타입이 'category'로 설정된다

#### Scenario: classifyURL은 skip을 반환하지 않는다
- **WHEN** 이전에 skip으로 분류되던 URL(ad, popup, login, signup, cart, checkout)을 classifyURL에 전달할 때
- **THEN** classifyURL은 NodeTypeSkip을 반환하지 않고 기본값인 'listing'을 반환한다

#### Scenario: 불필요한 URL 제외는 FilterChain이 담당
- **WHEN** URL이 'ad', 'popup', 'login', 'signup', 'cart', 'checkout'을 포함할 때
- **THEN** PathPatternFilter가 해당 URL을 필터링하여 BFS 큐에 추가하지 않는다

---

### Requirement: BFS로 사이트를 탐색한다 (Pioneer)
시스템은 너비 우선 탐색으로 사이트 링크를 순회해야 한다(SHALL). 링크 추출은 DOM 기반 파서를 사용해야 하며(SHALL), 링크 필터링은 필터 체인을 통해 수행해야 한다(SHALL).

#### Scenario: DOM 기반 링크 추출
- **WHEN** Pioneer가 HTML 페이지에서 링크를 추출할 때
- **THEN** DOM 기반 파서를 사용하여 링크와 DOM 위치 정보를 함께 추출한다

#### Scenario: FilterChain을 통한 링크 필터링
- **WHEN** 추출된 링크 목록을 필터링할 때
- **THEN** 필터 체인을 통해 도메인, 확장자, 경로 패턴, 중복 필터를 순차적으로 적용한다

#### Scenario: 이미 방문한 링크에 대한 엣지 생성
- **WHEN** FilterChain이 이미 방문한 링크를 필터링했을 때
- **THEN** 이미 방문한 링크에 대해 유효한 노드 ID가 있으면 현재 노드에서 해당 노드로의 엣지를 생성한다

#### Scenario: 복합 우선순위 계산
- **WHEN** 신규 링크를 BFS 큐에 추가할 때
- **THEN** 노드 타입 기반 우선순위에 DOM 위치 기반 보정값을 더한 복합 우선순위를 적용한다

---

### Requirement: 정규식 패턴을 패키지 레벨에서 컴파일한다
시스템은 URL 분류에 사용하는 정규식을 패키지 초기화 시 한 번만 컴파일해야 한다(SHALL).

#### Scenario: 숫자 ID 패턴 패키지 레벨 컴파일
- **WHEN** bot 패키지가 로드될 때
- **THEN** URL 분류에 사용하는 정규식이 패키지 레벨 변수로 한 번만 컴파일되어 매 호출 시 재컴파일하지 않는다
