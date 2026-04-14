## ADDED Requirements

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
