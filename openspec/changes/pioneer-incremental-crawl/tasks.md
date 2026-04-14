## 1. BFS 큐 로직 수정

- [x] 1.1 `pioneer.go`의 duplicate key 분기(line 220-236)에서 `queue.Push(&QueueItem{URL: link, URLHash: linkHash, Priority: priority})` 추가. `priority := NodeTypePriority(linkType)` 계산을 duplicate key 분기 이전으로 이동 (`linkType`는 이미 line 205에서 계산됨)
- [x] 1.2 `pioneer.go` line 161의 `nodesProcessed++`를 line 136의 `else` 블록(CreateNode 성공) 내부, `currentNodeID = newNode.ID` 직후로 이동. `GetNodeByHash` 성공(기존 노드 재방문) 시에는 카운터가 증가하지 않음

## 2. Stale edge 삭제

- [x] 2.1 sqlc 쿼리 추가: `ListEdgesBySiteNodes` — 특정 site의 노드가 from_node인 edge 목록 조회. `SELECT e.id, e.from_node_id, e.to_node_id FROM bot_graph_edges e JOIN bot_graph_nodes n ON e.from_node_id = n.id WHERE n.site_id = $1`
- [x] 2.2 sqlc 쿼리 추가: `DeleteEdgesByIDs` — ID 목록으로 edge 일괄 삭제. `DELETE FROM bot_graph_edges WHERE id = ANY($1::uuid[])`
- [x] 2.3 `sqlc generate` 실행하여 Go 코드 재생성
- [x] 2.4 `crawl()` 시작부에 `existingEdges` 맵 초기화: `ListEdgesBySiteNodes`로 해당 사이트의 모든 기존 edge를 `map[edgeKey]uuid.UUID`에 저장 (`edgeKey`는 `from_node_id:to_node_id` 문자열)
- [x] 2.5 `crawl()` 내 edge 생성 로직에서 `confirmedEdges` 집합에 해당 edgeKey 추가 (CreateEdge 성공 시 및 duplicate edge 시 모두)
- [x] 2.6 `crawl()` 종료 직전에 `visited` 맵의 값(uuid.UUID)들을 `visitedNodeIDs set[uuid.UUID]`로 변환한 뒤, `existingEdges` 중 `from_node_id`가 `visitedNodeIDs`에 포함되고 `confirmedEdges`에 없는 edge ID들을 `DeleteEdgesByIDs`로 삭제

## 3. 테스트

- [x] 3.1 `TestPioneerIncrementalCrawl` 추가: 첫 실행으로 depth 1까지 크롤 (maxNodes=3) → 재실행 시 기존 노드를 통과하여 depth 2까지 새 노드 발견 확인
- [x] 3.2 `TestPioneerStaleEdgeCleanup` 추가: 첫 실행에서 A→B edge 생성 → 테스트 서버에서 B 링크 제거 → 재실행 후 A→B edge 삭제 확인
- [x] 3.3 `TestPioneerMaxNodesCountsNewOnly` 추가: DB에 기존 노드 5개, maxNodes=3으로 실행 → 기존 노드 재방문은 카운트 안 하고 새 노드 3개 생성 확인
- [x] 3.4 기존 테스트 통과 확인: `go test ./internal/bot/... -count=1`
