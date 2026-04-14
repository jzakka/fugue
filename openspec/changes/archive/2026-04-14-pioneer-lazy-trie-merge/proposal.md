## Why

Pioneer의 `templatePath`는 순수 숫자 segment만 `{id}`로 치환하여 노드를 dedup한다. `/artworks/12345` 같은 경우는 잘 동작하지만, `/howto/search/AIart`, `/howto/search/8bit` 같은 비숫자 파라미터(검색어, 태그명, 유저 슬러그)는 각각 별도 노드로 생성된다. 현재 pixiv.net 크롤 결과에서 `/howto/search/` 아래에만 238개 중복 노드가 존재하며, 이는 그래프 시각화와 크롤 효율을 모두 저해한다.

단일 URL만 보고는 마지막 segment가 경로인지 파라미터인지 판단할 수 없다. 크롤된 데이터 전체의 통계(같은 prefix 아래 고유 자식 수)를 분석해야 정확한 판단이 가능하다.

## What Changes

- Pioneer 크롤 중에는 URL을 기존 방식 그대로 저장한다 (변경 없음)
- 크롤 완료 후 DB에 저장된 노드 URL들로 Path Trie를 빌드하고, 리프 노드이면서 형제 수가 임계값(기본 5)을 초과하는 레벨의 segment를 `{param}`으로 치환하여 머지한다
- 머지 대상은 **리프 노드만** 해당. 자식이 있는 segment는 경로로 판단하여 머지하지 않는다
- 머지 시 중복 노드를 하나로 통합하고, 관련 엣지도 재연결 + 중복 제거한다
- `make show-map` 또는 별도 커맨드로 후처리를 트리거한다

## Capabilities

### New Capabilities
(없음)

### Modified Capabilities
- `bot`: PathTrie 기반 URL 패턴 분석, 노드 머지 후처리, Pioneer 크롤 후 자동 실행, CLI merge 서브커맨드

## Impact

- `apps/api/internal/bot/` - 새 파일 `path_trie.go` 추가, Pioneer 또는 별도 커맨드에서 머지 호출
- `apps/api/internal/db/` - 노드 URL/해시 일괄 업데이트, 중복 엣지 삭제용 SQL 쿼리 추가
- `apps/api/cmd/bot-visualize/` 또는 `apps/api/cmd/bot/` - 머지 트리거 커맨드 연결
- DB `bot_graph_nodes` 테이블 - 기존 데이터의 url, url_hash 값이 머지 시 변경됨
