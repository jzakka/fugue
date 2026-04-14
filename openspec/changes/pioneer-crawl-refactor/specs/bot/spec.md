## MODIFIED Requirements

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
