## MODIFIED Requirements

### Requirement: BFS로 사이트를 탐색한다
시스템은 너비 우선 탐색으로 사이트 링크를 순회하며 설정 가능한 최대 깊이를 준수해야 한다(SHALL). BFS 탐색 로직은 페이지 조회 방법과 독립적으로 테스트 가능해야 한다(SHALL).

#### Scenario: 최대 깊이 제한 준수
- **WHEN** Pioneer가 최대 깊이 5로 루트에서 시작할 때
- **THEN** 깊이 6의 노드는 방문하지 않는다

#### Scenario: 부모 관계 추적
- **WHEN** Pioneer가 페이지 A에서 URL B를 발견할 때
- **THEN** 노드 B는 A를 부모로 하고 A보다 깊이가 1 증가한 상태로 생성된다

#### Scenario: Fetcher 인터페이스를 통한 페이지 조회
- **WHEN** BFS 탐색 중 페이지를 조회해야 할 때
- **THEN** Fetcher 인터페이스의 Fetch 메서드를 호출하여 페이지를 가져온다

#### Scenario: 테스트 시 FileFetcher 사용
- **WHEN** 단위 테스트에서 BFS 로직을 검증할 때
- **THEN** FileFetcher를 주입하여 파일 시스템 기반 fixture로 테스트한다

#### Scenario: 프로덕션 시 HTTPFetcher 사용
- **WHEN** 실제 크롤링을 수행할 때
- **THEN** HTTPFetcher를 주입하여 HTTP 요청으로 페이지를 가져온다

#### Scenario: HTTP 요청 시 10초 타임아웃 적용
- **WHEN** HTTPFetcher가 페이지를 조회할 때
- **THEN** 10초 내에 응답이 없으면 타임아웃 에러를 반환한다

#### Scenario: HTTP 리다이렉트 체인 처리
- **WHEN** HTTPFetcher가 http→https→www 같은 리다이렉트 체인을 만날 때
- **THEN** 최대 5번까지 리다이렉트를 따라가고 최종 페이지를 반환한다

#### Scenario: 리다이렉트 루프 방지
- **WHEN** HTTPFetcher가 5번을 초과하는 리다이렉트를 만날 때
- **THEN** "too many redirects" 에러를 반환한다

#### Scenario: HTTP 상태 코드 에러 처리
- **WHEN** HTTPFetcher가 404나 500 같은 에러 상태 코드를 받을 때
- **THEN** 상태 코드를 포함한 에러를 반환하여 상위 로직이 처리하게 한다

#### Scenario: 빈 응답 검증
- **WHEN** HTTPFetcher가 200 OK지만 응답 본문이 비어있는 경우
- **THEN** "empty response" 에러를 반환한다

#### Scenario: User-Agent 식별
- **WHEN** HTTPFetcher가 HTTP 요청을 보낼 때
- **THEN** "FugueBot/1.0 (+https://fugue.app)" User-Agent 헤더를 포함한다

#### Scenario: HTTP 에러 발생 시 로그 출력
- **WHEN** HTTPFetcher가 에러를 반환할 때
- **THEN** Pioneer는 에러를 로그로 출력하고 해당 URL을 건너뛴다

#### Scenario: HTTP 에러 발생 시 로그 출력
- **WHEN** HTTPFetcher가 에러를 반환할 때
- **THEN** Pioneer는 에러를 로그로 출력하고 해당 URL을 건너뛴다
