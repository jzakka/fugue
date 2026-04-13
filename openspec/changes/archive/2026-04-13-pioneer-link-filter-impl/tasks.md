## 0. 선행 조건 확인
- [x] 0.1 `pioneer-link-filter-interface` 변경이 구현되어 `crawler.Link`, `Selector`, `LinkFilter`, `FilterChain` 타입이 존재하는지 확인
- [x] 0.2 `go build ./apps/api/internal/bot/crawler/...` 및 `go build ./apps/api/internal/bot/...` 성공 확인

## 1. 헬퍼 함수 구현

- [x] 1.1 `link_filter.go` 파일 생성 및 패키지 선언, import 블록 작성
- [x] 1.2 `canonicalPath(urlStr string) string` 구현 — 트래킹 파라미터(utm_*, ref, fbclid, gclid) 제거, www 정규화, trailing slash 통일
- [x] 1.3 `VisitedLink` 구조체 정의 — `Link crawler.Link`, `NodeID uuid.UUID` 필드
- [x] 1.4 `semanticPriorityModifier(link crawler.Link) int` 구현 — footer/aside: -50, nav/header: -20, else: 0

## 2. DomainFilter 구현

- [x] 2.1 `DomainFilter` 구조체 정의 — `RootDomain string` 필드
- [x] 2.2 `Filter(links []crawler.Link) []crawler.Link` 메서드 구현 — `isSameDomain()` 래핑

## 3. ExtensionFilter 구현

- [x] 3.1 `ExtensionFilter` 구조체 정의
- [x] 3.2 `Filter(links []crawler.Link) []crawler.Link` 메서드 구현 — `hasExcludedExtension()` 래핑

## 4. PathPatternFilter 구현

- [x] 4.1 `defaultExcludePatterns` 변수 정의 — "ad", "popup", "login", "signup", "cart", "checkout"
- [x] 4.2 `PathPatternFilter` 구조체 정의 — `ExcludePatterns []string` 필드
- [x] 4.3 `Filter(links []crawler.Link) []crawler.Link` 메서드 구현 — `urlPathContains()` 사용, ExcludePatterns가 nil이면 defaultExcludePatterns 사용

## 5. CanonicalDedupFilter 구현

- [x] 5.1 `CanonicalDedupFilter` 구조체 정의 — `visited map[string]uuid.UUID`, `seen map[string]bool`, `LastVisited []VisitedLink`
- [x] 5.2 `Filter(links []crawler.Link) []crawler.Link` 메서드 구현 — `canonicalPath()` + `hashURL()` 기반 중복 제거, visited 맵 확인 후 LastVisited 기록

## 6. 단위테스트 작성

- [x] 6.1 `link_filter_test.go` 파일 생성 및 테스트 헬퍼(crawler.Link 생성 유틸) 작성
- [x] 6.2 `TestCanonicalURL` — 트래킹 파라미터 제거, www 정규화, trailing slash 통일 검증 (canonicalPath → canonicalURL로 rename, pioneer.go의 기존 canonicalPath와 충돌 방지)
- [x] 6.3 `TestSemanticPriorityModifier` — nav=-20, footer=-50, main=0 검증
- [x] 6.4 `TestDomainFilter` — 동일 도메인 통과, 외부 차단, www 정규화 검증
- [x] 6.5 `TestExtensionFilter` — .jpg/.css 제거, HTML 경로 통과 검증
- [x] 6.6 `TestPathPatternFilter` — /ad//popup/ 차단, 정상 경로 통과, 부분 매칭 방지 검증
- [x] 6.7 `TestCanonicalDedupFilter` — 동일 URL 중복 제거, canonical 중복 제거(utm 파라미터), LastVisited 기록 검증
- [x] 6.8 `TestFilterChain` — 체이닝 순서 보장, 빈 목록 처리 검증 (FilterChain은 `pioneer-link-filter-interface`에서 구현됨. 여기서는 구체 필터 조합 테스트만 수행)
- [x] 6.9 `go test ./apps/api/internal/bot/...` 실행하여 전체 테스트 통과 확인 (145 tests passed)
