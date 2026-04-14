## ADDED Requirements

### Requirement: PathTrie가 URL path segment를 트리 구조로 관리한다
PathTrie는 전체 URL에서 path를 추출하여 segment 단위로 트리에 삽입하고 관리해야 한다(SHALL). 각 트리 노드는 자식 노드 맵과 고유 자식 수를 추적해야 한다(SHALL).

#### Scenario: 전체 URL에서 path 추출 후 삽입
- **WHEN** `https://www.pixiv.net/howto/search/AIart`를 trie에 삽입할 때
- **THEN** scheme과 host를 제거하고 `howto` -> `search` -> `AIart` 순서로 트리 노드가 생성된다

#### Scenario: path-only URL 삽입
- **WHEN** `/artworks/{id}`를 trie에 삽입할 때
- **THEN** `artworks` -> `{id}` 순서로 트리 노드가 생성된다

#### Scenario: 같은 prefix 아래 여러 URL 삽입
- **WHEN** `/howto/search/AIart`, `/howto/search/8bit`, `/howto/search/watercolor`를 순서대로 삽입할 때
- **THEN** `search` 노드의 자식 수는 3이다

#### Scenario: 다른 prefix URL은 독립적
- **WHEN** `/howto/search/AIart`와 `/artworks/12345`를 삽입할 때
- **THEN** `howto`와 `artworks`는 루트의 별도 자식으로 존재한다

---

### Requirement: PathTrie가 리프 노드의 형제 수 기반으로 파라미터 레벨을 판별한다 (Leaf Explosion)
PathTrie는 특정 레벨의 리프 자식 수가 임계값을 초과하는지 판별해야 한다(SHALL). 자식이 있는(비리프) segment는 파라미터 후보에서 제외해야 한다(SHALL).

#### Scenario: 임계값 초과 리프 판별
- **WHEN** `/howto/search/` 아래에 6개의 서로 다른 리프 segment가 존재하고 임계값이 5일 때
- **THEN** `search` 노드 아래 레벨은 파라미터로 판별된다

#### Scenario: 임계값과 정확히 동일한 수
- **WHEN** `/howto/search/` 아래에 5개의 서로 다른 리프 segment가 존재하고 임계값이 5일 때
- **THEN** `search` 노드 아래 레벨은 파라미터로 판별되지 않는다

#### Scenario: 임계값 미달 시 파라미터 아님
- **WHEN** `/howto/search/` 아래에 3개의 리프 segment만 존재하고 임계값이 5일 때
- **THEN** `search` 노드 아래 레벨은 파라미터로 판별되지 않는다

#### Scenario: 자식이 있는 segment는 파라미터에서 제외
- **WHEN** `/aaa/` 아래에 `bbb`, `ccc`, `ddd`, `eee`, `fff`, `ggg` 6개가 있고, 그 중 `bbb` 아래에 `/aaa/bbb/254`가 존재할 때
- **THEN** `bbb`는 자식이 있으므로 파라미터 후보에서 제외되고, 나머지 리프(5개)만으로 파라미터 여부를 판단한다

#### Scenario: 모든 자식이 비리프이면 파라미터 아님
- **WHEN** `/api/` 아래에 `users/`, `posts/`, `comments/` 등 자식이 있는 6개 segment가 존재할 때
- **THEN** 모두 비리프이므로 파라미터로 판별되지 않는다

---

### Requirement: PathTrie가 subtree fingerprint 기반으로 mid-path 파라미터를 판별한다 (Mid-Path Parameterization)
PathTrie는 각 노드의 subtree fingerprint를 계산해야 한다(SHALL). 같은 부모 아래에서 동일한 fingerprint를 가진 형제가 임계값을 초과하면 해당 레벨을 파라미터로 판별해야 한다(SHALL). 단, subtree에 하나 이상의 static segment(`{}`로 감싸지지 않은 segment)가 포함된 경우에만 머지 대상으로 인정해야 한다(SHALL).

#### Scenario: 동일 suffix를 가진 형제 탐지
- **WHEN** `/tags/TAG1/artwork`, `/tags/TAG2/artwork`, ... `/tags/TAG6/artwork` 총 6개가 존재하고 임계값이 5일 때
- **THEN** `TAG1`~`TAG6`의 subtree fingerprint가 모두 `"artwork:∅"`로 동일하므로, `tags/` 아래 레벨은 파라미터로 판별된다

#### Scenario: 깊은 중첩 suffix도 탐지
- **WHEN** `/users/USER1/posts/recent`, `/users/USER2/posts/recent`, ... 6개가 존재하고 임계값이 5일 때
- **THEN** `USER*`의 subtree fingerprint가 모두 `"posts(recent:∅)"`로 동일하므로, `users/` 아래 레벨은 파라미터로 판별된다

#### Scenario: subtree가 parameterized-only이면 머지 대상에서 제외
- **WHEN** `/api/users/{id}`, `/api/posts/{id}`, ... 6개가 존재하고 임계값이 5일 때
- **THEN** subtree fingerprint는 동일하지만 subtree에 static segment가 없으므로(`{id}`만 존재) 머지 대상에서 제외된다

#### Scenario: subtree에 static intermediate + parameterized leaf가 혼재하면 머지 대상
- **WHEN** `/categories/CAT1/items/{id}`, `/categories/CAT2/items/{id}`, ... 6개가 존재하고 임계값이 5일 때
- **THEN** subtree에 static segment `items`가 포함되어 있으므로 머지 대상으로 인정된다

#### Scenario: 서로 다른 fingerprint를 가진 형제는 각각 별도 평가
- **WHEN** `/tags/TAG1/artwork`, `/tags/TAG2/artwork`, `/tags/TAG3/illustrations` 3개가 존재하고 임계값이 5일 때
- **THEN** fingerprint `"artwork:∅"` 그룹은 2개, `"illustrations:∅"` 그룹은 1개로, 어느 그룹도 임계값을 초과하지 않으므로 머지되지 않는다

#### Scenario: 임계값 미달
- **WHEN** `/tags/TAG1/artwork`, `/tags/TAG2/artwork` 2개만 존재하고 임계값이 5일 때
- **THEN** 동일 fingerprint 형제 수(2)가 임계값 이하이므로 머지되지 않는다

---

### Requirement: MergeTargets가 머지 대상 prefix 목록을 반환한다
PathTrie 분석 결과에서 머지가 필요한 prefix 경로, 해당 레벨의 파라미터 segment 목록, 그리고 suffix(mid-path의 경우)를 반환해야 한다(SHALL). Leaf explosion과 mid-path parameterization 모두 동일한 MergeTarget 구조로 반환해야 한다(SHALL).

#### Scenario: leaf explosion 머지 대상 반환
- **WHEN** `/howto/search/` 아래에 238개 리프가 있고 임계값이 5일 때
- **THEN** prefix `/howto/search`, 238개 파라미터 값, suffix 빈 문자열이 반환된다. 템플릿 URL은 `/howto/search/{param}`이 된다.

#### Scenario: mid-path 머지 대상 반환
- **WHEN** `/tags/TAG1/artwork`, `/tags/TAG2/artwork`, ... 6개가 존재하고 임계값이 5일 때
- **THEN** prefix `/tags`, 6개 파라미터 값(TAG1~TAG6), suffix `/artwork`가 반환된다. 템플릿 URL은 `/tags/{param}/artwork`가 된다.

#### Scenario: 머지 대상 없음
- **WHEN** 모든 prefix 아래 리프 수 및 동일 fingerprint 형제 수가 임계값 이하일 때
- **THEN** 빈 목록이 반환된다

#### Scenario: leaf explosion + mid-path 복합
- **WHEN** `/howto/search/` 아래에 10개 리프, `/tags/*/artwork` 패턴이 8개가 존재하고 임계값이 5일 때
- **THEN** 두 패턴 모두 머지 대상으로 반환된다

---

### Requirement: MergeNodes가 대상 노드들을 하나의 템플릿 노드로 통합한다
머지 대상으로 판별된 노드들을 DB에서 하나의 대표 노드로 통합해야 한다(SHALL). 대표 노드는 가장 먼저 생성된 노드여야 한다(SHALL). 하나의 prefix에 대한 머지 작업은 원자적으로 실행해야 한다(SHALL). 머지 순서는: (1) 머지 대상 노드를 참조하는 엣지 중 재연결 후 중복이 되는 것을 먼저 삭제, (2) 나머지 엣지를 대표 노드로 재연결, (3) self-loop 엣지 삭제, (4) 대표 노드 외 나머지 노드 삭제, (5) 대표 노드의 url과 url_hash를 `{param}` 템플릿 패턴으로 업데이트해야 한다(SHALL). 템플릿 URL은 leaf explosion의 경우 `Prefix/{param}`, mid-path의 경우 `Prefix/{param}Suffix` 형식이어야 한다(SHALL).

#### Scenario: leaf explosion 머지 동작
- **WHEN** `/howto/search/` 아래에 노드 A(생성 1일전), B(생성 2일전), C(생성 오늘)가 머지 대상일 때
- **THEN** 가장 먼저 생성된 B가 대표 노드로 선택되고, A와 C의 엣지가 B로 재연결되며, A와 C는 삭제되고, B의 url은 `/howto/search/{param}` 패턴으로 변경된다

#### Scenario: mid-path 머지 동작
- **WHEN** `/tags/TAG1/artwork`(노드 A), `/tags/TAG2/artwork`(노드 B), `/tags/TAG3/artwork`(노드 C) 등 6개가 머지 대상일 때
- **THEN** 가장 먼저 생성된 노드가 대표가 되고, 나머지 노드의 엣지가 대표로 재연결되며, 대표 노드의 url은 `/tags/{param}/artwork` 패턴으로 변경된다

#### Scenario: 엣지 재연결
- **WHEN** 노드 X -> A, 노드 Y -> C 엣지가 존재하고 A, C가 대표 노드 B로 머지될 때
- **THEN** X -> B, Y -> B 엣지로 재연결된다

#### Scenario: 재연결 시 중복 엣지 사전 제거
- **WHEN** X -> A, X -> B 엣지가 존재하고 A가 대표 노드 B로 머지될 때
- **THEN** X -> A 엣지를 X -> B로 재연결하면 기존 X -> B와 중복이므로, X -> A 엣지를 먼저 삭제한 후 재연결하지 않는다

#### Scenario: self-loop 엣지 제거
- **WHEN** 머지 전 A -> B 엣지가 있었고 A와 B가 같은 대표 노드로 머지될 때
- **THEN** self-loop 엣지는 삭제된다

#### Scenario: 대표 노드의 sample_url 보존
- **WHEN** 대표 노드의 sample_url이 존재할 때
- **THEN** 머지 후에도 대표 노드의 sample_url은 변경되지 않는다

---

### Requirement: Pioneer 크롤 완료 후 trie merge를 자동 실행한다
Pioneer의 사이트 크롤이 완료된 후, 해당 사이트의 노드에 대해 PathTrie 기반 머지를 자동으로 실행해야 한다(SHALL).

#### Scenario: 크롤 후 자동 머지
- **WHEN** Pioneer가 사이트 크롤을 완료할 때
- **THEN** 해당 사이트의 모든 노드 URL로 PathTrie를 빌드하고 머지를 실행한다

#### Scenario: 머지 결과 로깅
- **WHEN** 머지가 완료될 때
- **THEN** 머지된 prefix 수와 제거된 노드 수를 stdout에 출력한다

#### Scenario: 머지 대상 없으면 스킵
- **WHEN** PathTrie 분석 결과 머지 대상이 없을 때
- **THEN** 머지를 스킵하고 그 사실을 출력한다

#### Scenario: 이미 머지된 사이트에 재실행
- **WHEN** 머지가 완료된 사이트에 대해 머지를 다시 실행할 때
- **THEN** 추가 변경 없이 완료된다

---

### Requirement: CLI에서 merge 서브커맨드로 수동 머지를 실행할 수 있다
`go run cmd/bot/main.go merge <site>` 커맨드로 특정 사이트의 노드 머지를 수동 실행할 수 있어야 한다(SHALL). 임계값을 플래그로 지정할 수 있어야 한다(SHALL).

#### Scenario: 수동 머지 실행
- **WHEN** `go run cmd/bot/main.go merge pixiv.net` 커맨드를 실행할 때
- **THEN** pixiv.net 사이트의 노드에 대해 trie merge가 실행된다

#### Scenario: 커스텀 임계값 지정
- **WHEN** `go run cmd/bot/main.go merge pixiv.net --threshold 10` 커맨드를 실행할 때
- **THEN** 임계값 10으로 trie merge가 실행된다

#### Scenario: 존재하지 않는 사이트
- **WHEN** DB에 없는 사이트 도메인으로 merge 커맨드를 실행할 때
- **THEN** 에러 메시지를 출력하고 종료한다
