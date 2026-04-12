## MODIFIED Requirements

### Requirement: 그래프 노드와 엣지를 관리한다
시스템은 크롤링한 페이지를 노드로, 링크를 엣지로 저장하며 중복을 방지해야 한다.

#### Scenario: 새 URL 추가
- **WHEN** 크롤 중 URL을 발견했을 때
- **THEN** 시스템은 해당 사이트에 해당 URL이 없는 경우에만 노드를 생성한다

#### Scenario: 중복 URL 거부
- **WHEN** 이미 존재하는 URL을 다시 추가하려 할 때
- **THEN** 시스템은 중복을 거부하고 기존 노드를 유지한다

#### Scenario: 링크 관계 기록
- **WHEN** Pioneer가 BFS 크롤 중 페이지 A에서 페이지 B로의 링크를 발견할 때
- **THEN** 시스템은 B의 노드를 생성(또는 기존 노드를 조회)한 뒤 A에서 B로의 엣지를 생성한다

#### Scenario: 이미 방문한 노드로의 엣지 생성
- **WHEN** Pioneer가 페이지 A에서 이미 방문한 페이지 C로의 링크를 발견할 때
- **THEN** 시스템은 A에서 C로의 엣지를 생성한다 (중복 엣지는 무시된다)

#### Scenario: 중복 엣지 방지
- **WHEN** 같은 링크를 여러 번 발견했을 때
- **THEN** 시스템은 하나의 엣지만 유지한다

#### Scenario: 사이트의 모든 listing 페이지 조회
- **WHEN** 특정 사이트의 listing 타입 노드들을 조회할 때
- **THEN** 시스템은 해당하는 모든 노드를 빠르게 반환한다

---

### Requirement: URL 패턴으로 페이지 타입을 분류한다
시스템은 발견한 URL을 키워드 매칭, 경로 패턴, 쿼리 파라미터 분석을 통해 타입(listing, gallery, detail, category, skip)으로 분류해야 한다.

#### Scenario: listing 페이지 분류
- **WHEN** URL 경로가 'trending', 'popular', 'hot', 'featured', 'recent', 'explore' 키워드를 포함할 때
- **THEN** 노드 타입이 'listing'으로 설정된다

#### Scenario: detail 페이지 분류 (경로 패턴)
- **WHEN** URL 경로에 4자리 이상 숫자 ID 패턴이 있고 고우선순위 키워드가 없을 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: detail 페이지 분류 (쿼리 파라미터)
- **WHEN** URL 쿼리 파라미터에 id, illust_id 등 명시적 ID 키의 숫자 값이 포함되어 있을 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: detail 페이지 분류 (콘텐츠 단수형 경로)
- **WHEN** URL 경로에 '/artworks/', '/photos/', '/works/', '/illust/' 등 단수형 콘텐츠 세그먼트가 포함될 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: category 페이지 분류
- **WHEN** URL 경로가 'category', 'tag', 'genre', 'style', 'contest', 'event' 키워드를 포함할 때
- **THEN** 노드 타입이 'category'로 설정된다

#### Scenario: gallery 페이지 분류
- **WHEN** URL 경로가 'gallery', 'collection', 'album', 'showcase' 키워드를 포함할 때
- **THEN** 노드 타입이 'gallery'로 설정된다

#### Scenario: 불필요한 URL 제외
- **WHEN** URL이 'ad', 'popup', 'login', 'signup', 'cart', 'checkout' 키워드를 포함할 때
- **THEN** 해당 URL은 그래프에 추가되지 않는다

#### Scenario: 기본 분류
- **WHEN** URL이 어떤 패턴에도 매칭되지 않을 때
- **THEN** 노드 타입이 'listing'으로 설정된다

---

### Requirement: BFS로 사이트를 탐색한다 (Pioneer)
시스템은 너비 우선 탐색으로 사이트 링크를 순회하며 노드 수 제한을 준수해야 한다(SHALL). 탐색 중 발견한 링크 관계를 엣지로 기록해야 한다(SHALL).

#### Scenario: 최대 노드 수 제한 준수
- **WHEN** Pioneer가 최대 노드 수 제한으로 크롤을 시작할 때
- **THEN** 제한을 초과하는 노드는 처리하지 않는다

#### Scenario: 부모 관계 추적 및 엣지 생성
- **WHEN** Pioneer가 페이지 A에서 URL B를 발견할 때
- **THEN** 노드 B를 생성(또는 기존 노드를 조회)한다
- **AND** A에서 B로의 엣지가 생성된다

#### Scenario: Fetcher 인터페이스를 통한 페이지 조회
- **WHEN** BFS 탐색 중 페이지를 조회해야 할 때
- **THEN** Fetcher 인터페이스의 Fetch 메서드를 호출하여 페이지를 가져온다

#### Scenario: 테스트 시 FileFetcher 사용
- **WHEN** 단위 테스트에서 BFS 로직을 검증할 때
- **THEN** FileFetcher를 주입하여 파일 시스템 기반 fixture로 테스트한다

#### Scenario: 프로덕션 시 HTTPFetcher 사용
- **WHEN** 실제 크롤링을 수행할 때
- **THEN** HTTPFetcher를 주입하여 HTTP 요청으로 페이지를 가져온다
