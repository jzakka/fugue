## 1. Canonical path 함수

- [ ] 1.1 `canonicalPath(urlStr string) string` 함수 구현: (1) URL 파싱 후 scheme+host+path만 추출 (쿼리/fragment 제거), (2) path 내 순수 숫자 세그먼트를 `{id}`로 치환. 반환값은 scheme+host 포함 (예: `https://www.pixiv.net/artworks/{id}`)
- [ ] 1.2 `canonicalPath` 단위 테스트 추가: 쿼리 제거, 숫자 ID 치환, slug 보존, 혼합 문자열 보존, 다중 숫자 세그먼트, 루트 URL 보존 등 시나리오 검증
- [ ] 1.3 기존 `hashURL()` 함수가 `canonicalPath()`의 결과를 해시하도록 변경

## 2. DB 마이그레이션 + sqlc

- [ ] 2.1 마이그레이션 파일 생성: `bot_graph_nodes`에 `sample_url TEXT` 컬럼 추가 (nullable, 기존 데이터는 NULL)
- [ ] 2.2 sqlc 쿼리 업데이트: `CreateNode`에 `sample_url` 파라미터 추가, `GetNodeByHash`/`GetNodeByURL`/`ListNodesBySite`/`ListNodesByType`/`ListAllNodesForGraph`의 SELECT에 `sample_url` 포함
- [ ] 2.3 `sqlc generate` 실행하여 Go 코드 재생성

## 3. Pioneer 크롤 루프 수정

- [ ] 3.1 `crawl()` 루프에서 노드 생성 시 `url` 필드에 canonical path를, `sample_url` 필드에 원본 URL을 저장하도록 수정. `classifyURL()`과 `fetchHTML()`은 원본 URL을 계속 사용
- [ ] 3.2 링크 순회 루프에서도 자식 노드 생성 시 동일하게 canonical path + sample_url 적용
- [ ] 3.3 `visited` 맵의 키가 canonical hash를 사용하도록 변경 (`hashURL` 내부에서 `canonicalPath`를 호출하므로 자동 적용되는지 확인)

## 4. Harvester 수정

- [ ] 4.1 `executeNode`에서 `node.Url` 대신 `node.SampleUrl`을 사용하여 HTML fetch 및 스크립트 실행 — `fetchHTML(ctx, node.SampleUrl)`, `executor.Execute(ctx, ..., node.SampleUrl)`로 변경
- [ ] 4.2 `findRootNode`에서 `GetNodeByURL(site.RootUrl)` 대신 `GetNodeByHash(canonicalPath(site.RootUrl))`를 사용하도록 변경하여 canonical URL 기반으로 루트 노드를 찾도록 수정

## 5. 시각화 호환

- [ ] 5.1 `visualize.Node` 구조체에 `SampleURL` 필드 추가, `FetchGraphData`에서 `sample_url` 조회
- [ ] 5.2 D3 template tooltip에서 `sample_url`을 표시하도록 수정 (canonical path는 `url` 필드로 별도 표시)

## 6. 테스트 + 검증

- [ ] 6.1 Pioneer 통합 테스트에 중복 path dedup 검증 추가: 쿼리만 다른 URL, 숫자 ID만 다른 URL이 같은 노드로 합쳐지는지 확인
- [ ] 6.2 Harvester 테스트에서 `sample_url` 기반 fetch가 동작하는지 확인
- [ ] 6.3 기존 테스트 통과 확인
- [ ] 6.4 `make pioneer SITE=pixiv` 재실행 후 노드 수 감소 확인 (수동 검증)
