## 1. DrainTree 자료구조

- [x] 1.1 `apps/api/internal/bot/drain.go` 생성: `DrainTree`, `DrainCluster` 구조체 정의. DrainTree는 depth→clusters 트리 구조 (first-segment 라우팅 제거 — depth-2 leaf explosion 지원)
- [x] 1.2 `DrainTree.Insert(rawURL string)` 메서드: URL을 segment로 분해 → depth 분류 → 기존 클러스터와 유사도 비교 → 합류 또는 새 클러스터 생성
- [x] 1.3 `DrainCluster.Template()` 메서드: 클러스터 내 URL들에서 가변 위치를 `{param}`으로 치환한 템플릿 반환
- [x] 1.4 `DrainTree.MergeTargets(countThreshold int)` 메서드: count threshold 초과 클러스터에서 MergeTarget 목록 반환. mid-path 클러스터는 static segment 후처리 검증 적용

## 2. Static Segment 후처리 검증

- [x] 2.1 `hasStaticAfterWildcard(template []string) bool` 헬퍼: 템플릿에서 wildcard 위치 이후에 static segment(`{}` 미포함)가 있는지 확인
- [x] 2.2 `MergeTargets()`에서 mid-path 클러스터(wildcard가 마지막이 아닌 경우)에 대해 static segment 검증 적용. 실패 시 머지 대상에서 제외

## 3. MergeTarget 확장 + 머지 로직 연결

- [x] 3.1 `MergeTarget` 구조체에 `Suffix string` 필드 추가
- [x] 3.2 `trie_merge.go`의 `RunTrieMerge` → `RunDrainMerge`로 이름 변경. 내부에서 DrainTree 사용
- [x] 3.3 `mergeOnePrefix()`에서 victim 노드 매칭: URL path가 `Prefix + "/" + paramValue + Suffix`인 노드
- [x] 3.4 템플릿 URL 생성: `Prefix + "/{param}" + Suffix`

## 4. 기존 코드 정리

- [x] 4.1 `path_trie.go` 삭제 (DrainTree로 대체)
- [x] 4.2 `path_trie_test.go` 삭제
- [x] 4.3 `cmd/bot/main.go`에서 `RunTrieMerge` 호출을 `RunDrainMerge`로 변경

## 5. 테스트

- [x] 5.1 `drain_test.go` 생성: Insert, Template 추출, 유사도 계산 단위 테스트
- [x] 5.2 Leaf explosion 탐지 테스트: `/howto/search/*` 패턴, 임계값 경계값
- [x] 5.3 Mid-path 탐지 테스트: `/tags/*/artwork` 패턴, 다중 suffix, 깊은 중첩(`/users/*/posts/recent`)
- [x] 5.4 오탐 방지 테스트: `/api/users/{id}` vs `/api/posts/{id}` static segment 검증 실패 확인
- [x] 5.5 Depth 분리 테스트: depth가 다른 URL이 별도 그룹으로 처리되는지 확인
- [x] 5.6 멱등성 테스트: 이미 머지된 데이터에 재실행 시 변경 없음

## 6. 통합 + 검증

- [x] 6.1 기존 Pioneer 테스트 통과 확인 (`go test ./internal/bot/...`)
- [x] 6.2 실제 DB 데이터로 Drain 머지 실행 및 결과 검증 (`merge pixiv.net`)
