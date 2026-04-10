## ADDED Requirements

### Requirement: BFS로 사이트를 탐색한다
시스템은 너비 우선 탐색으로 사이트 링크를 순회하며 설정 가능한 최대 깊이를 준수해야 한다.

#### Scenario: 최대 깊이 제한 준수
- **WHEN** Pioneer가 최대 깊이 5로 루트에서 시작할 때
- **THEN** 깊이 6의 노드는 방문하지 않는다

#### Scenario: 부모 관계 추적
- **WHEN** Pioneer가 페이지 A에서 URL B를 발견할 때
- **THEN** 노드 B는 A를 부모로 하고 A보다 깊이가 1 증가한 상태로 생성된다

### Requirement: URL 패턴으로 페이지 타입을 분류한다
시스템은 발견한 URL을 키워드 매칭을 통해 타입(listing, gallery, detail, category, skip)으로 분류해야 한다.

#### Scenario: listing 페이지 분류
- **WHEN** URL이 'trending', 'popular', 'hot', 'featured', 'recent' 키워드를 포함할 때
- **THEN** 노드 타입이 'listing'으로 설정된다

#### Scenario: detail 페이지 분류
- **WHEN** URL이 숫자 ID 패턴을 가지고 고우선순위 키워드가 없을 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: 불필요한 URL 제외
- **WHEN** URL이 'ad', 'popup', 'login', 'signup', 'cart'를 포함할 때
- **THEN** 해당 URL은 그래프에 추가되지 않는다

### Requirement: 엄격한 도메인 경계를 적용한다
시스템은 루트 도메인과 일치하는 링크만 따라가야 하며 서브도메인은 제외해야 한다.

#### Scenario: 동일 도메인 허용
- **WHEN** 링크가 dribbble.com → www.dribbble.com일 때
- **THEN** 정규화된 도메인이 일치하여 링크를 따라간다

#### Scenario: 서브도메인 차단
- **WHEN** 링크가 dribbble.com → ads.dribbble.com일 때
- **THEN** 서브도메인이 달라 링크를 거부한다

#### Scenario: 외부 도메인 차단
- **WHEN** 링크가 dribbble.com → twitter.com일 때
- **THEN** 도메인이 달라 링크를 거부한다

### Requirement: 파일 확장자를 제외한다
시스템은 미디어/문서 확장자로 끝나는 URL을 건너뛰어야 한다.

#### Scenario: 이미지 파일 제외
- **WHEN** URL이 .jpg, .png, .gif, .webp, .svg로 끝날 때
- **THEN** 해당 URL은 그래프에 추가되지 않는다

#### Scenario: 정적 자산 제외
- **WHEN** URL이 .css, .js, .json, .xml로 끝날 때
- **THEN** 해당 URL은 그래프에 추가되지 않는다

### Requirement: 페이지 타입별로 우선순위를 적용한다
시스템은 높은 우선순위의 노드 타입을 낮은 우선순위보다 먼저 처리해야 한다.

#### Scenario: listing 페이지를 먼저 처리
- **WHEN** BFS 큐에 listing 페이지와 detail 페이지가 모두 있을 때
- **THEN** listing 페이지를 detail 페이지보다 먼저 방문한다

### Requirement: 유효한 스크립트를 재사용한다
시스템은 기존 스크립트를 검증하여 검증이 통과하면 AI 생성을 건너뛰어야 한다.

#### Scenario: 스크립트 검증 통과
- **WHEN** 기존 스크립트가 페이지의 예상 아이템 중 70% 이상을 추출할 때
- **THEN** 스크립트를 재사용하고 AI 호출 없이 재사용 카운터를 증가시킨다

#### Scenario: 스크립트 검증 실패
- **WHEN** 기존 스크립트가 예상 아이템의 70% 미만을 추출할 때
- **THEN** AI를 통해 새 스크립트를 생성하고 기존 스크립트를 대체한다

### Requirement: AI로 파싱 스크립트를 생성한다
시스템은 페이지 HTML과 URL을 AI에 전달하여 파싱 스크립트를 생성해야 한다.

#### Scenario: 새 노드 타입용 스크립트 생성
- **WHEN** 특정 (사이트, 노드타입) 조합에 대한 스크립트가 없을 때
- **THEN** AI를 호출하여 스크립트를 생성하고 비용을 추적한다

### Requirement: Pioneer 실행 통계를 기록한다
시스템은 각 Pioneer 실행의 메트릭을 기록해야 한다.

#### Scenario: 실행 완료 기록
- **WHEN** Pioneer 실행이 완료될 때
- **THEN** 발견한 노드 수, 생성한 스크립트 수, 재사용한 스크립트 수, AI 비용을 기록한다
