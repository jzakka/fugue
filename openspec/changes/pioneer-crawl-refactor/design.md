## Context

Pioneer의 `crawl()` 메서드(약 190줄)는 링크 추출, 도메인 필터링, 확장자 필터링, 중복 체크, 엣지 생성, 노드 생성, 큐 푸시를 하나의 for 루프 안에서 처리한다. 세 가지 선행 변경이 개별 관심사를 모듈화했다:

1. **pioneer-link-filter-interface**: `LinkFilter` 인터페이스 + `FilterChain` 구조체 정의
2. **pioneer-link-extractor-selectors**: DOM 기반 `ExtractLinksWithSelectors()` 함수
3. **pioneer-link-filter-impl**: `DomainFilter`, `ExtensionFilter`, `PathPatternFilter`, `CanonicalDedupFilter` 구현체

이 변경은 위 세 모듈을 `crawl()`에 통합하고, 더 이상 필요 없는 인라인 코드를 정리하는 최종 단계이다.

## Goals / Non-Goals

**Goals:**
- `crawl()`의 링크 처리 파이프라인을 `FilterChain.Apply()`로 단순화
- regex 기반 `parseLinks()`를 DOM 기반 `ExtractLinksWithSelectors()`로 교체
- `classifyURL()`을 순수한 타입 분류 함수로 정리 (skip 판정 책임 제거)
- 컴파일 시 정규식 생성 오버헤드 제거 (패키지 레벨 변수로 추출)
- 기존 테스트를 새 동작에 맞게 업데이트

**Non-Goals:**
- FilterChain 인터페이스나 개별 필터 구현 변경 (선행 변경에서 완료)
- `crawl()`의 BFS 로직이나 노드 생성 로직 변경
- 외부 동작(API, CLI) 변경
- 성능 최적화 (이 단계에서는 동작 보존에 집중)

## Decisions

### 1. FilterChain을 crawl() 시작 시 한 번 생성

**결정**: `crawl()` 메서드 진입 시 FilterChain 인스턴스를 한 번 만들고 루프에서 재사용한다.

```go
dedupFilter := NewCanonicalDedupFilter(visited)
filterChain := NewFilterChain(
    NewDomainFilter(rootDomain),
    NewExtensionFilter(),
    NewPathPatternFilter(),
    dedupFilter,
)
```

**근거**: CanonicalDedupFilter는 visited 맵을 참조해야 하므로 루프 밖에서 생성하되, visited 맵의 상태가 루프 내에서 변경되므로 포인터 기반 참조가 필요하다. DomainFilter, ExtensionFilter, PathPatternFilter는 stateless이므로 한 번 생성으로 충분하다.

**대안 고려**: 루프 매 이터레이션마다 FilterChain을 새로 생성하는 방안 — 불필요한 오버헤드이므로 기각.

### 2. 방문 노드 엣지 처리를 dedupFilter.LastVisited로 분리

**결정**: `CanonicalDedupFilter`가 필터링 시 이미 방문한 링크를 `LastVisited` 슬라이스에 기록하고, 메인 루프에서 이를 읽어 엣지만 생성한다.

```go
// 방문 노드 → 엣지만 생성
for _, vl := range dedupFilter.LastVisited {
    if vl.NodeID != uuid.Nil {
        p.graphRepo.CreateEdge(ctx, ...)
    }
}
// 신규 노드 → 노드 생성 + 엣지 생성 + 큐 푸시
for _, link := range filteredLinks { ... }
```

**근거**: 기존 코드에서 `visited[linkHash]`를 직접 체크하던 로직을 FilterChain 내부로 이동하면, 필터링 결과에 "왜 제외되었는지"의 사이드채널 정보가 필요하다. `LastVisited` 필드가 이 역할을 한다.

**대안 고려**: FilterChain이 방문 노드도 결과에 포함하되 플래그로 구분하는 방안 — 필터의 의미가 불명확해지므로 기각.

### 3. classifyURL()에서 skip 로직 제거

**결정**: `classifyURL()`은 순수하게 타입 분류만 수행한다. skip 판정은 `PathPatternFilter`가 담당한다.

**근거**: 단일 책임 원칙. classifyURL은 "이 URL은 어떤 타입인가?"만 답하고, "이 URL을 방문해야 하는가?"는 FilterChain이 답한다. 이를 통해 classifyURL의 테스트도 단순화된다.

**대안 고려**: classifyURL에서 skip 패턴을 유지하고 FilterChain에서도 중복 체크하는 방안 — 이중 책임으로 유지보수 복잡성 증가, 기각.

### 4. regexp.MustCompile을 패키지 레벨로 추출

**결정**: `classifyURL()` 내부에서 매 호출마다 `regexp.MustCompile()`을 실행하던 것을 패키지 레벨 변수로 추출한다.

```go
var numericIDPattern = regexp.MustCompile(`^\d+$`)
var pathNumericIDPattern = regexp.MustCompile(`/\d{4,}(?:/|$)`)
```

**근거**: `regexp.MustCompile()`은 컴파일 비용이 있으며, classifyURL은 발견된 링크마다 호출된다. 패키지 레벨 변수로 한 번만 컴파일하면 된다.

### 5. 복합 우선순위 계산

**결정**: `priority := NodeTypePriority(linkType) + semanticPriorityModifier(link)`로 타입 기반 우선순위에 의미론적 가중치를 추가한다.

**근거**: 같은 타입이라도 URL 패턴에 따라 우선순위 차이가 있을 수 있다 (예: /trending vs /page/3). semanticPriorityModifier가 이를 보완한다.

## Risks / Trade-offs

- **[Risk] CanonicalDedupFilter와 visited 맵의 동기화**: dedupFilter가 외부 visited 맵을 참조하므로, 맵 업데이트 타이밍이 중요하다 → **Mitigation**: 각 이터레이션에서 filteredLinks 처리 후 visited 맵을 즉시 업데이트하고, 다음 이터레이션에서 dedupFilter가 최신 상태를 참조하도록 보장.
- **[Risk] parseLinks() 삭제로 인한 하위 호환성**: 다른 코드에서 parseLinks를 호출하는 곳이 있을 수 있다 → **Mitigation**: grep으로 사용처를 확인하고, crawler 패키지에 없는 경우에만 삭제.
- **[Risk] 세 가지 의존 변경이 미완료 상태에서 적용 시도**: 컴파일 에러 발생 → **Mitigation**: 이 변경의 tasks는 의존 변경 완료 후에만 실행. CI에서 빌드 검증.
- **[Trade-off] FilterChain 추상화 레이어 추가**: 코드 복잡도가 약간 증가하지만, 테스트 가능성과 확장성이 크게 향상된다.
- **[Risk] 동시 삽입 방어 패턴 보존**: 현재 `crawl()`의 노드 생성 시 `duplicate key`/`unique constraint` 에러를 처리하여 기존 노드를 조회하고 엣지를 생성하는 방어 로직(pioneer.go 213-231행)이 존재한다. 리팩터링 후에도 이 패턴을 그대로 유지해야 한다.
