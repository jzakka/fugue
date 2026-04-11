## 1. Repository Layer

- [x] 1.1 Add GetNodeByURL method to GraphRepository interface for finding root node
- [x] 1.2 Add GetNodeByURL SQL query to db/queries/bot.sql for sqlc generation
- [x] 1.3 Run sqlc generate to create Go code from SQL queries
- [x] 1.4 Implement GetNodeByURL in GraphRepositoryImpl (wraps generated sqlc method)

## 2. BFS Core Logic

- [x] 2.1 Create BFS queue structure to hold nodes per level (can reuse or extend priority_queue.go)
- [x] 2.2 Implement visited set using map[uuid.UUID]bool for cycle detection
- [x] 2.3 Create findRootNode helper function to locate root URL node from site.root_url
- [x] 2.4 Implement harvestBFS method to replace current harvest implementation
- [x] 2.5 Add level-based traversal loop that processes nodes level by level

## 3. Priority Sorting within Levels

- [x] 3.1 Extract sortNodesByPriority to work on a slice of nodes (already exists, verify it's reusable)
- [x] 3.2 Apply sortNodesByPriority to each BFS level before processing
- [x] 3.3 Ensure same-type nodes maintain discovery order after sorting

## 4. Node Processing

- [x] 4.1 For each processed node, call GetEdgesByNode to fetch children (use existing method)
- [x] 4.2 Extract to_node_id from returned BotGraphEdge slice
- [x] 4.3 Filter children to exclude already-visited nodes
- [x] 4.4 Add unvisited children to next level queue
- [x] 4.5 Maintain existing executeNode, fetchHTML, and pipeline integration logic

## 5. Error Handling

- [x] 5.1 Return error if root node is not found (suggest Pioneer re-run)
- [x] 5.2 Handle GetEdgesByNode errors gracefully (log and continue)
- [x] 5.3 Ensure BFS stops when queue is empty or max depth is reached (if applicable)

## 6. Testing

- [x] 6.1 Write unit test for BFS traversal with simple graph (A→B, A→C, B→D)
- [x] 6.2 Write unit test for cycle detection (A→B→C→A)
- [x] 6.3 Write unit test for priority sorting within levels (listing before detail at same depth)
- [x] 6.4 Write unit test for root node not found error
- [x] 6.5 Add integration test using MockGraphRepository with edge data
- [x] 6.6 Verify existing harvester_test.go still passes with new BFS logic

## 7. Documentation

- [x] 7.1 Update internal/bot/README.md to document BFS traversal approach
- [x] 7.2 Add code comments explaining BFS algorithm and visited set
- [x] 7.3 Document GetNodeByURL method in repository interface
