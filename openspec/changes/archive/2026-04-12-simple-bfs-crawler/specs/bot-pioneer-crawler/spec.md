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
