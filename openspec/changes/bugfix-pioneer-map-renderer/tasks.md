## 1. Pioneer Edge 생성 수정

- [x] 1.1 `Pioneer.crawl()` BFS 루프에서 `GetNodeByHash`/`CreateNode` 반환값을 `_`로 버리지 않고 `currentNodeID`를 변수에 캡처하도록 리팩터링
- [x] 1.2 `visited` 맵을 `map[string]bool`에서 `map[string]uuid.UUID`로 변경하여 방문 완료 노드의 ID를 보존 — 루트 노드 등록 시에도 `visited[rootHash] = rootNodeID` 형태로 저장
- [x] 1.3 BFS 링크 순회 루프에서 각 자식 링크에 대해: (a) 미방문이면 `CreateNode`로 자식 노드 생성 후 `childNodeID` 획득, `CreateEdge(currentNodeID, childNodeID)` 호출, `visited[hash] = childNodeID` 저장 후 큐에 push (b) 이미 방문이면 `visited[hash]`에서 `childNodeID` 조회 후 `CreateEdge(currentNodeID, childNodeID)` 호출 (큐 push 없음)
- [x] 1.4 Edge 생성 실패 시 로깅만 하고 크롤을 계속하도록 에러 처리 추가 (중복 edge는 DB의 ON CONFLICT DO NOTHING으로 무시)
- [x] 1.5 Pioneer 테스트(`pioneer_test.go`)에 edge 생성 검증 케이스 ��가: 부모→자식 edge 생성 확인, 이미 방문한 노드로의 edge 생성 확인, 중복 edge 무시 확인

## 2. URL 분류 로직 개선

- [x] 2.1 `classifyURL()` 함수에 쿼리 파라미터 기반 detail 분류 추가: `?id=\d+`, `?illust_id=\d+` 등 명시적 ID 파라미터 인식 (`?p=`는 pagination이므로 제외)
- [x] 2.2 콘텐츠 단수형 경로 세그먼트 기반 detail 분류 추가: `/artworks/`, `/photos/`, `/works/`, `/illust/` 등
- [x] 2.3 category 키워드 확장: `contest`, `event` 추가
- [x] 2.4 분류 로직의 단위 테스트 추가 및 기존 `TestClassifyURL` 케이스 업데이트: Pixiv, Unsplash 등 실제 URL 패턴을 포함하고, 기존 테스트 기대값이 새 로직과 일치하는지 확인

## 3. D3 시각화 수정

- [x] 3.1 Go `visualize.Edge` JSON 직렬화 시 `source`/`target` 필드를 추가하는 커스텀 MarshalJSON 구현 (`from_node_id`/`to_node_id`는 유지)
- [x] 3.2 D3 template(`template.html`)의 `forceLink` 설정이 `source`/`target` 필드를 사용하도록 확인
- [x] 3.3 `ticked()` 콜백에서 수동 `data.nodes.find()` 탐색을 제거하고, D3 forceLink가 자동 resolve한 `d.source.x`/`d.source.y`를 사용하도록 수정
- [x] 3.4 노드 타입별 색상 체계 적용: listing=blue, gallery=purple, detail=green, category=amber, unknown=gray — script 구현 여부는 stroke 스타일로 표현
- [x] 3.5 Legend 패널을 새 색상 체계에 맞게 업데이트

## 4. 검증

- [ ] 4.1 `make pioneer SITE=pixiv` 재실행 후 DB에 edge가 생성되는지 확인 (DB 연결 필요 — 수동 검증)
- [ ] 4.2 `make show-map` 실행 후 생성된 `graph.html`에서 edge가 보이는지, 노드 타입별 색상이 구분되는지 브라우저에서 확인 (수동 검증)
