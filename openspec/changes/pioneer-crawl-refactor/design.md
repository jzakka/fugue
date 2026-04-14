## Context

Pioneer의 `crawl()` 메서드(약 190줄)는 링크 추출, 도메인 필터링, 확장자 필터링, 중복 체크, 엣지 생성, 노드 생성, 큐 푸시를 하나의 for 루프 안에서 처리한다. 이미 구현 완료된 모듈:

- `crawler/link_extractor.go`: `ExtractLinksWithSelectors()` — DOM 기반 링크 추출, `[]crawler.Link` 반환
- `link_filter.go`: `FilterChain`, `DomainFilter`, `ExtensionFilter`, `PathPatternFilter`, `CanonicalDedupFilter`, `semanticPriorityModifier()`

또한 `harvester.go`의 `fetchHTML()`은 `"not implemented"` stub이므로 Pioneer의 fetch 로직을 공유 함수로 추출하여 양쪽에서 사용해야 한다.

## Goals / Non-Goals

**Goals:**
- `crawl()`의 링크 처리 파이프라인을 `FilterChain.Apply()`로 단순화
- regex 기반 `parseLinks()`를 DOM 기반 `ExtractLinksWithSelectors()`로 교체
- `classifyURL()`을 순수한 타입 분류 함수로 정리 (skip 판정 책임 제거)
- `regexp.MustCompile()` 패키지 레벨 변수로 추출
- Pioneer의 `fetchHTML` 로직을 공유 함수로 추출하여 Harvester stub 해소
- 기존 테스트를 새 동작에 맞게 업데이트

**Non-Goals:**
- FilterChain 인터페이스나 개별 필터 구현 변경 (이미 완료)
- `crawl()`의 BFS 로직이나 노드 생성 로직 변경
- 외부 동작(API, CLI) 변경
- Harvester의 BFS 실행 로직 변경 (fetchHTML 연결만)

## Decisions

### 1. FilterChain을 crawl() 시작 시 한 번 생성

**결정**: `crawl()` 메서드 진입 시 FilterChain 인스턴스를 한 번 만들고 루프에서 재사용.

**근거**: CanonicalDedupFilter는 visited 맵을 포인터로 참조하므로 루프 밖에서 생성해도 최신 상태를 반영한다. DomainFilter, ExtensionFilter, PathPatternFilter는 stateless이므로 한 번 생성으로 충분.

### 2. 방문 노드 엣지 처리를 dedupFilter.LastVisited로 분리

**결정**: `CanonicalDedupFilter`가 필터링 시 이미 방문한 링크를 `LastVisited` 슬라이스에 기록. 메인 루프에서 이를 읽어 엣지만 생성.

**근거**: 기존 코드에서 `visited[linkHash]`를 직접 체크하던 로직을 FilterChain 내부로 이동하면, "왜 제외되었는지" 사이드채널 정보가 필요하다. `LastVisited` 필드가 이 역할을 한다.

### 3. classifyURL()에서 skip 로직 제거

**결정**: `classifyURL()`은 순수하게 타입 분류만 수행. skip 판정은 `PathPatternFilter`가 담당.

**근거**: 단일 책임 원칙. classifyURL은 "이 URL은 어떤 타입인가?"만 답하고, "이 URL을 방문해야 하는가?"는 FilterChain이 답한다.

### 4. regexp.MustCompile을 패키지 레벨로 추출

**결정**: `classifyURL()` 내부의 `regexp.MustCompile()` 호출을 패키지 레벨 변수로 추출.

**근거**: classifyURL은 발견된 링크마다 호출되며, regexp 컴파일은 비용이 있다. 한 번만 컴파일하면 된다.

### 5. 복합 우선순위 계산

**결정**: `priority := NodeTypePriority(linkType) + semanticPriorityModifier(link)`로 타입 기반 우선순위에 DOM 위치 기반 보정값을 추가.

**근거**: 같은 타입이라도 DOM 위치에 따라 가치가 다르다 (footer/aside의 링크 < main content의 링크).

### 6. 공유 fetchHTML 함수 추출

**결정**: Pioneer의 fetch 로직을 패키지 레벨 공유 함수로 추출하여 Pioneer와 Harvester가 동일한 HTTP 설정(사이즈 제한, 리다이렉트 제한, 타임아웃, User-Agent)으로 HTML을 가져오도록 한다.

**근거**: Pioneer의 fetch 로직은 5MB 사이즈 제한, 5-redirect 제한, 10s 타임아웃, User-Agent 설정을 포함하는 견고한 구현이다. Harvester에 동일 로직을 복제하는 것은 DRY 위반.

## Risks / Trade-offs

- **[Risk] CanonicalDedupFilter와 visited 맵 동기화**: dedupFilter가 외부 visited 맵을 참조하므로 맵 업데이트 타이밍 중요 → **Mitigation**: 각 이터레이션에서 filteredLinks 처리 후 visited 맵 즉시 업데이트.
- **[Risk] parseLinks() 삭제 영향**: 다른 코드에서 호출하는 곳이 있을 수 있음 → **Mitigation**: grep으로 사용처 확인 후 삭제.
- **[Risk] fetchHTML 추출 시 Pioneer 메서드 시그니처 변경**: Pioneer의 fetchHTML이 Pioneer 구조체에 의존하는 경우 → **Mitigation**: http.Client만 파라미터로 받는 순수 함수로 추출.
- **[Trade-off] FilterChain 추상화 레이어 추가**: 코드 복잡도 약간 증가하지만 테스트 가능성과 확장성이 크게 향상.
- **[Risk] 동시 삽입 방어 패턴 보존**: 노드 생성 시 `duplicate key` 에러 처리 로직을 리팩터링 후에도 유지해야 함.
