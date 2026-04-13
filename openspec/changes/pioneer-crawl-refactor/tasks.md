## 1. classifyURL 정리

- [ ] 1.1 `pioneer.go`에서 패키지 레벨 정규식 변수 추출: `` var numericIDPattern = regexp.MustCompile(`^\d+$`) `` 및 `` var pathNumericIDPattern = regexp.MustCompile(`/\d{4,}(?:/|$)`) `` 선언하고, classifyURL 내부의 인라인 `regexp.MustCompile()` 호출을 이 변수로 교체
- [ ] 1.2 `classifyURL()`에서 skip 패턴 블록 제거 (lines 391-397: `skipPatterns` 배열과 for 루프). classifyURL은 더 이상 `NodeTypeSkip`을 반환하지 않음
- [ ] 1.3 `domain.go`의 `NodeTypeSkip` 상수 및 `NodeTypePriority`의 skip case는 보존한다 (하위 호환성 및 다른 코드에서 참조 가능성). `NodeTypeSkip`은 더 이상 classifyURL에서 반환되지 않지만 타입 시스템에는 유지.
- [ ] 1.4 `urlPathContains()` 헬퍼는 다른 패턴 매칭에서 사용하므로 유지

## 2. crawl() 메서드 리팩터링

- [ ] 2.1 `pioneer.go`에 `"strings"` (이미 존재) 및 `"github.com/chungsanghwa/fugue/apps/api/internal/bot/crawler"` import 추가
- [ ] 2.2 `crawl()` 시작부에 FilterChain 인스턴스 생성: `dedupFilter := NewCanonicalDedupFilter(visited)`, `filterChain := NewFilterChain(NewDomainFilter(rootDomain), NewExtensionFilter(), NewPathPatternFilter(), dedupFilter)`
- [ ] 2.3 `parseLinks(html, item.URL)` 호출을 `crawler.ExtractLinksWithSelectors(strings.NewReader(html), item.URL)`로 교체
- [ ] 2.4 인라인 필터 로직 제거: `isSameDomain()`, `hasExcludedExtension()` 호출 및 `visited[linkHash]` 직접 체크를 `filterChain.Apply(crawlerLinks)`로 대체
- [ ] 2.5 기존 헬퍼 함수 보존 확인: `isSameDomain()`, `hasExcludedExtension()`, `urlPathContains()`는 필터 구현체가 래핑 호출하므로 삭제하지 않음
- [ ] 2.6 `if nodeType == NodeTypeSkip { continue }` 체크 제거 (PathPatternFilter가 큐 진입 전에 처리)
- [ ] 2.7 방문 노드 엣지 생성 로직 추가: `dedupFilter.LastVisited` 순회하며 유효한 NodeID가 있는 경우 `p.graphRepo.CreateEdge(ctx, ...)` 호출
- [ ] 2.8 신규 링크의 우선순위 계산 변경: `linkType := classifyURL(link.URL)`, `priority := NodeTypePriority(linkType) + semanticPriorityModifier(link)`. 여기서 `link`는 `crawler.Link` 타입이다.
- [ ] 2.9 기존 duplicate key 방어 패턴 보존: 신규 링크 노드 생성 시 unique constraint 에러 발생 시 기존 노드 조회 → visited 맵 업데이트 → 엣지 생성 로직 유지

## 3. helpers.go 정리

- [ ] 3.1 `helpers.go`에서 `parseLinks()` 함수 삭제 (DOM 기반 `ExtractLinksWithSelectors`로 완전 대체)
- [ ] 3.2 `toNullString()` 헬퍼는 유지. 사용하지 않는 import 정리 (`regexp` 등)

## 4. 테스트 업데이트

- [ ] 4.1 `pioneer_test.go`의 `TestClassifyURL`에서 skip 패턴 테스트 케이스 업데이트: `NodeTypeSkip` 기대값을 `NodeTypeListing`으로 변경 (ad, popup, login, signup, cart, checkout URL들). 각 URL이 classifyURL의 detail/gallery/category 패턴에 매칭되지 않고 기본값 listing으로 분류됨을 검증.
- [ ] 4.2 `go test ./apps/api/internal/bot/...` 실행하여 모든 테스트 통과 확인
- [ ] 4.3 `go build ./apps/api/...` 실행하여 컴파일 에러 없음 확인
