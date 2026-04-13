## Context

Pioneer 크롤러의 `link_extractor.go`에는 현재 unexported 함수 `extractLinks(body io.Reader, baseURL string) ([]string, error)`가 있다. 이 함수는 `golang.org/x/net/html` 패키지로 DOM을 순회하면서 `<a href>` 요소를 찾아 절대 URL 문자열 배열을 반환한다.

Link Filter Chain 기능을 위해 각 링크가 DOM 내에서 어떤 시맨틱 영역(`<nav>`, `<footer>`, `<main>`, `<aside>` 등)에 위치하는지 알아야 한다. 이를 통해 네비게이션 링크(중복 많음, 낮은 가치), 본문 링크(높은 가치), 푸터 링크(법적/유틸리티) 등을 구분하여 필터링 정책을 적용할 수 있다.

`pioneer-link-filter-interface` change에서 `Link`와 `Selector` 타입이 정의될 예정이며, 본 change는 해당 타입을 사용하는 실제 추출 로직을 구현한다.

## Goals / Non-Goals

**Goals:**
- DOM 순회 시 ancestor element를 추적하여 각 링크의 시맨틱 위치 정보를 제공
- 기존 `extractLinks()` 함수와 `BFSCrawler` 동작에 영향 없이 새 함수 추가
- 충분한 테스트 커버리지 확보 (nav, footer, main, aside, 중첩 구조)

**Non-Goals:**
- `BFSCrawler`가 `ExtractLinksWithSelectors`를 사용하도록 변경하는 것 (별도 change에서 처리)
- CSS class/id 기반 세밀한 selector 해석 (시맨틱 태그명만 추적)
- 링크 필터링 정책 자체의 구현 (이 change는 데이터 수집만 담당)

## Decisions

### 1. Ancestor 스택 방식: 재귀 호출 시 슬라이스 전달

DOM 순회를 위한 `visit` 함수에 `[]Selector` 파라미터를 추가하여 현재까지의 ancestor 경로를 전달한다. 각 ElementNode 진입 시 태그명을 스택에 push하고, 자식 노드 순회 후 자동으로 pop된다 (재귀 호출이므로 별도 pop 불필요).

**대안 검토:**
- 글로벌 스택 + 수동 push/pop: 코드 복잡도 증가, pop 누락 위험
- Parent pointer 역추적: `<a>` 발견 시 부모를 역방향 탐색. 동일 결과이나 매 링크마다 O(depth) 역추적 필요

**선택 근거:** 재귀 호출의 파라미터로 전달하면 스택 관리가 Go 런타임에 위임되어 안전하고 간결하다.

### 2. Selector 기록 범위: 시맨틱 HTML5 태그 + 주요 structural 태그

모든 HTML 태그를 기록하면 noise가 많아진다. 다음 태그만 selector로 기록한다:
- 시맨틱 태그: `nav`, `main`, `aside`, `footer`, `header`, `article`, `section`
- Structural 태그: `body`, `div` (class 포함 시)

`div`는 class 속성이 있을 때만 기록하여 의미 있는 컨텍스트를 제공한다.

**대안 검토:**
- 모든 태그 기록: 과도한 noise, 필터링 시 오히려 복잡
- 시맨틱 태그만: `div.content` 같은 유용한 정보 누락

**선택 근거:** 시맨틱 태그 + class가 있는 div를 기록하면 실용적인 수준의 위치 정보를 제공한다.

**다중 class 처리**: `<div class="a b c">`와 같이 다중 class가 있는 경우, class 속성의 전체 값을 그대로 사용한다. 예: `Selector{TagName: "div", Class: "a b c"}`. 첫 번째 class만 추출하는 것은 context 정보 손실이므로 기각.

### 3. 기존 함수 보존

`extractLinks()`는 변경하지 않는다. `BFSCrawler`가 이미 사용 중이며, selector 정보가 불필요한 기존 흐름에 오버헤드를 추가할 이유가 없다.

### 4. Link/Selector 타입 의존성

`pioneer-link-filter-interface` change에서 정의하는 `Link`, `Selector` 타입을 import하여 사용한다. 같은 `crawler` 패키지 내에 정의될 예정이므로 별도 import 없이 바로 참조 가능하다.

## Risks / Trade-offs

- **[메모리 오버헤드]** 각 링크마다 ancestor 슬라이스를 복사하므로 깊은 DOM에서 메모리 사용량이 증가한다 → 실제 웹 페이지의 시맨틱 태그 깊이는 일반적으로 5-10 수준이므로 무시할 수 있는 수준이다
- **[타입 의존성]** `Link`/`Selector` 타입이 `pioneer-link-filter-interface`에서 먼저 정의되어야 컴파일 가능하다 → 구현 순서를 지켜야 하며, tasks에 의존성 명시
- **[selector 정확도]** `div.content`처럼 class 기반 selector는 사이트마다 다르다 → 필터 체인에서 시맨틱 태그 우선 매칭, class는 보조 정보로 활용
- **[주의] html.Parse의 암시적 노드 생성**: `golang.org/x/net/html`의 `html.Parse()`는 fragment 입력 시 `<html>`, `<head>`, `<body>` 노드를 암시적으로 생성한다. `isSelectorTarget()`이 `<html>`, `<head>`에 대해 false를 반환하도록 하여 이들이 selector에 포함되지 않도록 한다. 테스트에서 이 동작을 검증해야 한다.
