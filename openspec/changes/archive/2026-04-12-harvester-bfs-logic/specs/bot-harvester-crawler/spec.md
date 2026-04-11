## MODIFIED Requirements

### Requirement: 매 실행마다 전체 그래프를 순회한다
시스템은 BFS 알고리즘을 사용하여 엣지를 따라 그래프를 레벨별로 순회하며, 방문한 노드를 추적해야 한다.

#### Scenario: 루트에서 BFS 시작
- **WHEN** Harvester가 시작될 때
- **THEN** 사이트의 루트 URL 노드를 큐에 추가하고 BFS 순회를 시작한다

#### Scenario: 엣지 기반 순회
- **WHEN** 노드를 처리할 때
- **THEN** 시스템은 해당 노드에서 연결된 모든 링크를 조회하여 자식 노드 목록을 가져온다

#### Scenario: 레벨별 순회
- **WHEN** 루트 노드(깊이 0)가 3개의 자식 노드를 가질 때
- **THEN** 루트 노드를 먼저 처리한 후 3개의 자식 노드(깊이 1)를 순서대로 처리한다

#### Scenario: 방문한 노드는 재방문하지 않음
- **WHEN** 노드 A가 이미 방문되었고 다른 경로에서 A를 다시 발견할 때
- **THEN** 노드 A를 큐에 추가하지 않고 건너뛴다

### Requirement: 타입 우선순위로 노드를 정렬한다
시스템은 BFS 레벨 내에서 listing > gallery > category > detail 순으로 노드를 처리해야 한다.

#### Scenario: 레벨 내 우선순위 정렬
- **WHEN** 깊이 1에 listing 노드 2개와 detail 노드 10개가 있을 때
- **THEN** listing 노드 2개를 먼저 처리한 후 detail 노드 10개를 처리한다

#### Scenario: 같은 타입은 발견 순서 유지
- **WHEN** 깊이 2에 detail 노드 5개가 있을 때
- **THEN** 엣지 발견 순서대로 5개 노드를 처리한다

## ADDED Requirements

### Requirement: 시작 노드를 자동으로 선택한다
시스템은 사이트의 루트 URL에 해당하는 노드를 BFS 시작점으로 사용해야 한다.

#### Scenario: 루트 URL 노드 시드 선택
- **WHEN** 사이트의 루트 URL에 해당하는 노드가 존재할 때
- **THEN** 해당 노드를 BFS의 시작 노드로 선택한다

#### Scenario: 루트 노드 부재 시 오류
- **WHEN** 루트 URL에 해당하는 노드가 없을 때
- **THEN** 오류를 반환하고 Harvester 실행을 중단한다

### Requirement: 순환 참조를 방지한다
시스템은 방문한 노드를 추적하여 그래프 내 순환(cycle)이 있어도 무한 루프에 빠지지 않아야 한다.

#### Scenario: 순환 그래프에서 재방문 차단
- **WHEN** 노드 A → B → C → A 순환 구조에서 A를 다시 발견할 때
- **THEN** A는 이미 방문되었으므로 큐에 추가하지 않는다

#### Scenario: 서로 다른 경로에서의 중복 발견
- **WHEN** 노드 A에서 C로, 노드 B에서도 C로 가는 링크가 있을 때
- **THEN** C는 처음 발견 시에만 큐에 추가되고 두 번째 발견 시에는 건너뛴다

### Requirement: 자식 노드가 없는 경우를 처리한다
시스템은 outgoing edges가 없는 노드를 정상적으로 처리해야 한다.

#### Scenario: 리프 노드 처리
- **WHEN** 노드에 연결된 자식 노드가 없을 때
- **THEN** 자식 노드 추가 없이 다음 노드로 진행한다
