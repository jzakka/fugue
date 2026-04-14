## Why

Pioneer의 `canonicalPath()`는 순수 숫자 segment만 `{id}`로 치환하여 크롤 시점에 dedup한다. 하지만 비숫자 파라미터(태그명, 검색어, 유저 슬러그 등)는 각각 별도 노드로 생성된다.

현재 두 가지 유형의 중복이 존재한다:

1. **Leaf explosion**: `/howto/search/AIart`, `/howto/search/8bit`, ... — 같은 prefix 아래 리프 노드 폭발
2. **Mid-path parameterization**: `/tags/TAG1/artwork`, `/tags/TAG2/artwork`, ... — 경로 중간에 파라미터가 위치하고 뒤에 고정 suffix가 오는 패턴

기존 `pioneer-lazy-trie-merge`는 leaf explosion만 탐지 가능했고, mid-path 패턴은 놓쳤다. Subtree fingerprint 확장을 검토했으나 복잡도가 높고, 로그 파싱 분야에서 검증된 Drain 알고리즘이 두 패턴을 통합적으로 처리할 수 있음을 발견했다.

## What Changes

- 크롤 완료 후 DB에 저장된 노드 URL들을 Drain 알고리즘(고정 깊이 parse tree + 유사도 기반 클러스터링)으로 분석하여 가변 segment를 `{param}`으로 치환
- Drain의 핵심 구조: (1) path depth로 1차 분류, (2) prefix segment로 tree 탐색, (3) 같은 클러스터 내 토큰별 유사도 비교, (4) 가변 토큰을 `{param}`으로 치환
- 기존 `path_trie.go`의 PathTrie를 DrainTree로 교체. `trie_merge.go`의 머지 로직(대표 노드 선택, 엣지 재연결, 중복 제거)은 재사용
- 머지 후처리에 static segment 검증을 추가하여 오탐 방지

## Capabilities

### Modified Capabilities
- `bot`: Drain 기반 URL 패턴 분석, static segment 검증, 노드 머지 후처리, Pioneer 크롤 후 자동 실행, CLI merge 서브커맨드

## Impact

- `apps/api/internal/bot/path_trie.go` → `drain.go`로 교체 (DrainTree, DrainCluster 구조체)
- `apps/api/internal/bot/trie_merge.go` → DrainTree 클러스터 결과 기반으로 MergeTarget 생성 로직 변경. 머지 실행 로직은 재사용
- `apps/api/internal/bot/path_trie_test.go` → `drain_test.go`로 교체 (leaf + mid-path + 오탐 방지 통합 테스트)
- `apps/api/cmd/bot/main.go` → 기존 merge 서브커맨드 유지, 내부 호출만 DrainTree로 변경
