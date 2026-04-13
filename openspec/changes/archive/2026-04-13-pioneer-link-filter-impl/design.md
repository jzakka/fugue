## Context

Pioneer 크롤러는 BFS 탐색 중 발견한 링크를 필터링하기 위해 `isSameDomain()`, `hasExcludedExtension()`, `urlPathContains()` 등의 함수를 `pioneer.go`에 직접 구현하고 있다. 이 로직들은 크롤 루프 안에 인라인으로 호출되어 있어 개별 테스트가 어렵고, 사이트별 필터 구성 변경이 불가능하다.

pioneer-link-filter-interface 변경에서 `LinkFilter` 인터페이스와 `crawler.Link` 타입이 정의될 예정이며, 이 변경은 해당 인터페이스의 4개 구체 구현체를 제공한다.

## Goals / Non-Goals

**Goals:**
- 기존 `pioneer.go`의 필터링 함수들을 `LinkFilter` 인터페이스 구현체로 래핑
- URL 정규화(canonical path) 기반 중복 제거 필터 신규 구현
- 시맨틱 우선순위 보정 헬퍼 함수 구현
- 각 필터와 헬퍼 함수에 대한 포괄적 단위테스트
- 필터 체이닝을 통한 조합 가능한 아키텍처

**Non-Goals:**
- 기존 `pioneer.go`의 크롤 루프를 필터 체인으로 교체하는 것 (별도 변경에서 수행)
- 사이트별 필터 구성 파일/DB 관리
- 필터 성능 벤치마크 및 최적화

## Decisions

### 1. 파일 구조: 단일 파일에 모든 필터 구현

**결정**: `link_filter.go`에 4개 필터 + 헬퍼를, `link_filter_test.go`에 모든 테스트를 배치한다.

**근거**: 필터들이 모두 같은 `LinkFilter` 인터페이스를 구현하고, 공유 헬퍼(`canonicalPath`, `hashURL`)를 사용하므로 같은 파일에 두는 것이 응집도가 높다. 필터 수가 4개로 제한적이므로 파일 분리 오버헤드가 더 크다.

**대안**: 필터별 파일 분리 (`domain_filter.go`, `extension_filter.go` 등) - 필터가 10개 이상으로 늘어날 경우 재고.

### 2. 기존 함수 래핑 vs 재구현

**결정**: DomainFilter, ExtensionFilter, PathPatternFilter는 기존 `pioneer.go` 함수(`isSameDomain`, `hasExcludedExtension`, `urlPathContains`)를 내부적으로 호출한다.

**근거**: 기존 함수들은 이미 테스트된 로직이며, 중복 구현은 버그 발산 위험이 있다. 래핑 패턴으로 인터페이스 호환성만 확보한다.

**대안**: 기존 함수를 필터 내부로 이동 후 원본 삭제 - 크롤 루프 교체 변경과 동시에 하는 것이 안전.

### 3. CanonicalDedupFilter의 상태 공유 전략

**결정**: `visited map[string]uuid.UUID`는 크롤 루프와 공유하는 참조 맵이고, `seen map[string]bool`은 필터 내부 전용 canonical 해시 셋이다. `LastVisited []VisitedLink`는 필터 호출 후 크롤 루프가 읽어서 엣지 생성에 사용한다.

**근거**: `visited` 맵은 이미 크롤 루프가 중복 방지용으로 관리하고 있으므로, 같은 맵 참조를 공유해야 일관성이 유지된다. `LastVisited`는 출력 채널 역할로, 필터가 "이 링크는 이미 방문됨"을 보고하되 필터 결과에서는 제외하는 패턴이다.

**키 체계 차이**: `visited` 맵은 원본 URL의 `hashURL(url)` 값을 키로 사용하고, `seen` 맵은 정규화된 `hashURL(canonicalPath(url))` 값을 키로 사용한다. 두 맵의 키 체계가 다르므로, `visited`에서는 정확한 URL 매칭만 가능하고, `seen`에서는 canonical 중복(예: UTM 파라미터 차이)까지 감지한다. `crawl-refactor` 변경에서 해시 전략 통일 여부를 결정해야 한다.

### 4. canonicalPath() 정규화 규칙

**결정**: 다음 파라미터를 제거한다: `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`, `ref`, `fbclid`, `gclid`. Host에서 `www.` 접두어를 제거하고, trailing slash를 통일한다.

**근거**: 주요 트래킹 파라미터만 제거하여 과도한 정규화로 인한 false positive를 방지한다. 크로스미디어 큐레이션 플랫폼 특성상 아트워크 URL의 쿼리 파라미터(예: `?page=2`, `?id=123`)는 보존해야 한다.

### 5. semanticPriorityModifier() 반환값

**결정**: footer/aside 위치 링크는 -50, nav/header 위치 링크는 -20, 그 외(main, article 등)는 0을 반환한다.

**근거**: BFS 우선순위 큐에 더해지는 보정값으로, footer의 광고/법적 링크와 nav의 반복 링크 우선순위를 낮춘다. 본문 링크는 보정 없이 기본 우선순위를 유지한다.

### 6. VisitedLink 구조체 (구현 세부사항)

`VisitedLink`는 이미 방문된 URL이 감지될 때 원본 `crawler.Link`와 기존 노드의 `uuid.UUID`를 포함하는 구조체이다. CanonicalDedupFilter의 `LastVisited` 필드를 통해 크롤 루프에 보고된다.

## Risks / Trade-offs

- **[Risk] visited 맵 동시 접근** → Pioneer는 단일 고루틴 BFS이므로 현재 동시성 문제 없음. 병렬화 시 sync.RWMutex 추가 필요.
- **[Risk] canonicalPath 과소 정규화** → 트래킹 파라미터 목록이 불완전할 수 있음. 실제 크롤 데이터로 추가 파라미터 발견 시 목록 확장.
- **[Trade-off] 기존 함수 래핑 방식** → 코드 중복은 없지만 pioneer.go에 대한 의존성 유지. 향후 크롤 루프 교체 시 함수를 필터 내부로 이동 가능.
- **[Trade-off] LastVisited 필드의 뮤터블 상태** → 필터가 상태를 가지므로 호출 순서 중요. 매 Filter() 호출 전 LastVisited를 리셋하는 규약으로 해결.
- **[Risk] classifyURL()과의 skip 패턴 중복**: PathPatternFilter의 `defaultExcludePatterns`와 `classifyURL()`의 skip 패턴이 동일한 목록("ad", "popup", "login", "signup", "cart", "checkout")이다. `crawl-refactor` 변경에서 `classifyURL()`의 skip 로직이 제거될 때까지 이중 필터링이 발생하나, 기능적으로 문제없다 (idempotent).
