## 1. PathTrie 자료구조

- [x] 1.1 `apps/api/internal/bot/path_trie.go` 생성: PathTrie 구조체, Insert, isLeaf 메서드 구현
- [x] 1.2 MergeTargets 메서드 구현: 리프 형제 수 > 임계값인 prefix 목록 반환 (비리프 자식 제외)
- [x] 1.3 `apps/api/internal/bot/path_trie_test.go` 작성: 삽입, 리프 판별, 임계값 경계값, 비리프 제외 테스트

## 2. DB 머지 로직

- [x] 2.1 `apps/api/db/queries/bot.sql`에 머지용 SQL 쿼리 추가: ListNodeURLsBySite, ListEdgesReferencingNodes, DeleteSelfLoopEdges, DeleteNodesByIDs, UpdateNodeURLAndHash
- [x] 2.2 sqlc 코드 생성 실행 (`sqlc generate`)
- [x] 2.3 `apps/api/internal/bot/trie_merge.go` 생성: DB 노드 로드 -> trie 빌드 -> 머지 대상 추출 -> DB 머지 실행하는 `RunTrieMerge(ctx, siteID, threshold)` 함수 구현
- [x] 2.4 머지 로직 테스트: 실제 DB 데이터로 통합 검증 (329→75 노드, 멱등성 확인, root-level 보호 추가)

## 3. Pioneer 통합

- [x] 3.1 `cmd/bot/main.go`의 pioneerCmd에서 크롤 완료 후 `RunTrieMerge` 호출 추가
- [x] 3.2 `cmd/bot/main.go`에 `merge` 서브커맨드 추가 (사이트 도메인 + --threshold 플래그)

## 4. 검증 (Phase 1)

- [x] 4.1 기존 Pioneer 테스트 통과 확인 (`go test ./internal/bot/...`) — 161 tests passed
- [x] 4.2 실제 DB 데이터로 수동 머지 실행 (`merge pixiv.net` → 2 prefixes, 253 nodes removed → root-level 보호 후 재검증 완료)

## 5. Subtree Fingerprint 기반 Mid-Path 탐지

- [ ] 5.1 `path_trie.go`에 `Fingerprint() string` 메서드 구현: 재귀적으로 자식을 정렬하여 canonical string 생성 (leaf → `"∅"`, non-leaf → `"name(fp)|name(fp)|..."`)
- [ ] 5.2 `path_trie.go`에 `hasStaticSegment(node) bool` 헬퍼 구현: subtree 내 `{}`로 감싸지지 않은 segment가 하나라도 있으면 true
- [ ] 5.3 `MergeTarget` 구조체에 `Suffix string` 필드 추가. 기존 leaf explosion 결과는 Suffix를 빈 문자열로 설정
- [ ] 5.4 `MergeTargets()` 메서드 확장: 기존 leaf explosion 탐지 후, 추가로 동일 fingerprint 형제 > threshold && static segment 검증을 통과하는 mid-path 머지 대상도 수집
- [ ] 5.5 `path_trie_test.go`에 mid-path 탐지 테스트 추가: `tags/TAG*/artwork` 패턴 탐지, 깊은 중첩(`users/USER*/posts/recent`), static 검증 실패(`api/users/{id}` vs `api/posts/{id}`), fingerprint 그룹별 독립 평가, 임계값 경계값

## 6. Mid-Path 머지 로직

- [ ] 6.1 `trie_merge.go`의 `mergeOnePrefix` 리팩터링: `MergeTarget.Suffix`를 반영하여 victim 노드 매칭 로직 수정 (URL path = `Prefix + "/" + paramValue + Suffix`)
- [ ] 6.2 템플릿 URL 생성 로직 수정: `Prefix + "/{param}" + Suffix` 형식
- [ ] 6.3 mid-path 머지 통합 테스트: 실제 DB 데이터 또는 mock으로 `tags/TAG*/artwork` 패턴 머지 검증 (엣지 재연결, 중복 제거, 템플릿 URL 생성)

## 7. 검증 (Phase 2)

- [ ] 7.1 전체 테스트 통과 확인 (`go test ./internal/bot/...`)
- [ ] 7.2 실제 DB 데이터로 mid-path 머지 실행 및 결과 검증
