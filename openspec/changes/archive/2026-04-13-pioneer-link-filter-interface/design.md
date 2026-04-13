## Context

Pioneer 크롤러(`apps/api/internal/bot/pioneer.go`)의 `crawl()` 메서드 안에서 링크 필터링이 인라인으로 처리되고 있다:

1. `isSameDomain(link, rootDomain)` — 도메인 검증
2. `hasExcludedExtension(link)` — 파일 확장자 필터링
3. `classifyURL(link)` → `NodeTypeSkip` 체크 — URL 패턴 기반 스킵

이 로직들은 `for _, link := range links` 루프 안에 하드코딩되어 있어, 새로운 필터 추가 시 Pioneer 코어 로직을 직접 수정해야 한다. 또한 현재 `extractLinks`(`crawler` 패키지)와 `parseLinks`(`bot` 패키지)가 `[]string`만 반환하므로 DOM 구조 정보가 유실된다.

**현재 패키지 구조:**
- `crawler` 패키지: `link_extractor.go`(extractLinks), `url_utils.go`(normalizeURL, makeAbsoluteURL 등)
- `bot` 패키지: `pioneer.go`(crawl, parseLinks), `domain.go`(NodeType 등), `interfaces.go`(AIClient 등)

**중복 함수 주의:**
- `crawler/url_utils.go`에도 `isSameDomain()` (37행)과 `shouldSkipURL()` (83행)이 이미 존재하며, `bot/pioneer.go`의 동명 함수와 기능이 중복된다.
- `crawler/bfs_crawler.go`의 BFS 루프(114-137행)에서도 동일한 필터링 패턴이 인라인으로 사용 중이다.
- 후속 변경에서 이들의 통합/제거 방향을 검토해야 한다.

## Goals / Non-Goals

**Goals:**
- DOM 구조 정보를 포함하는 `Link` 타입을 `crawler` 패키지에 정의한다
- `LinkFilter` 인터페이스를 `bot` 패키지에 정의하여 필터의 계약을 표준화한다
- `FilterChain`을 통해 여러 필터를 순차 적용하는 구조를 제공한다
- 타입과 인터페이스만 정의하고 컴파일 가능한 상태를 유지한다

**Non-Goals:**
- 구체적인 필터 구현 (DomainFilter, ExtensionFilter 등은 `filter-impl`에서)
- 기존 `extractLinks`/`parseLinks` 반환 타입 변경 (이는 `link-extractor`에서)
- `Pioneer.crawl()`의 리팩터링 (이는 `crawl-refactor`에서)
- 테스트 코드 작성 (인터페이스/타입만이므로 테스트할 로직 없음)

**parseLinks/extractLinks 이중구조 참고:**
- `bot/helpers.go`의 `parseLinks()`는 정규식 기반(`href=["']...`)으로 링크를 추출하며, `crawler/link_extractor.go`의 `extractLinks()`는 `golang.org/x/net/html` DOM 파서를 사용한다. Pioneer의 `crawl()`은 현재 `parseLinks()`를 사용 중이다. `link-extractor` 변경에서 `parseLinks`를 DOM 기반으로 교체할 예정이다.

## Decisions

### 1. `Link` 타입을 `crawler` 패키지에 배치

`Link`는 DOM에서 추출된 링크의 구조화된 표현이다. `crawler` 패키지가 HTML 파싱 및 링크 추출을 담당하므로 여기에 배치하는 것이 자연스럽다.

**대안**: `bot` 패키지에 배치 → `crawler`가 `bot`을 import해야 하는 순환 의존 발생. 기각.

```go
// crawler/link.go
type Selector struct {
    TagName string
    ID      string
    Class   string
}

type Link struct {
    URL       string
    Selectors []Selector // DOM ancestor path: root → <a>
}
```

`Selectors`는 DOM 루트에서 `<a>` 태그까지의 조상 경로를 저장한다. 이를 통해 향후 "nav 안의 링크는 제외" 같은 CSS selector 기반 필터가 가능해진다.

### 2. `LinkFilter` 인터페이스를 `bot` 패키지에 배치

필터는 크롤링 정책(도메인 제한, 확장자 제외 등)을 표현하며, 이는 `bot`의 비즈니스 로직이다. `crawler`는 순수 HTML 파싱에 집중하고, 필터링 정책은 `bot`이 소유한다.

```go
// bot/link_filter.go
type LinkFilter interface {
    Filter(links []crawler.Link) []crawler.Link
}
```

단일 메서드 인터페이스로 Go 관용적 설계를 따른다. 입력 슬라이스를 받아 필터링된 슬라이스를 반환하는 함수형 스타일.

### 3. `FilterChain`으로 순차 적용

```go
type FilterChain struct {
    filters []LinkFilter
}

func NewFilterChain(filters ...LinkFilter) *FilterChain
func (c *FilterChain) Apply(links []crawler.Link) []crawler.Link
```

`Apply`는 등록된 필터를 순서대로 적용한다. 각 필터의 출력이 다음 필터의 입력이 된다. `FilterChain` 자체는 `LinkFilter`를 구현하지 않는다 — `Apply` 메서드명으로 역할을 명시적으로 구분한다.

**대안**: `FilterChain`도 `LinkFilter`를 구현 → 체인의 중첩이 가능해지나 현 단계에서는 불필요한 복잡도. 필요 시 향후 추가.

### 4. 파일 배치

| 파일 | 패키지 | 내용 |
|------|--------|------|
| `apps/api/internal/bot/crawler/link.go` | `crawler` | `Selector`, `Link` 타입 |
| `apps/api/internal/bot/link_filter.go` | `bot` | `LinkFilter` 인터페이스, `FilterChain` 구조체 |

## Risks / Trade-offs

- **`Selectors` 필드의 메모리 오버헤드** → 링크 수가 수백 수준이므로 무시 가능. 향후 대규모 크롤링 시 재평가.
- **인터페이스만 정의, 구현 없음** → 컴파일은 되지만 사용 불가. `filter-impl` 변경이 빠르게 뒤따라야 한다. 리스크가 아닌 의도적 분리.
- **`bot` → `crawler` 단방향 의존** → 이미 존재하는 패턴. 순환 의존 리스크 없음.
