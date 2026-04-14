## ADDED Requirements

### Requirement: 크롤된 URL 집합에서 가변 segment를 자동 탐지한다
시스템은 크롤된 URL 집합의 통계적 분석을 통해 가변 segment를 `{param}`으로 치환해야 한다(SHALL). leaf explosion과 mid-path parameterization을 모두 탐지해야 한다(SHALL). 탐지 임계값은 설정 가능해야 한다(SHALL).

#### Scenario: leaf explosion 탐지 (같은 prefix 아래 리프 폭발)
- **WHEN** `/howto/search/AIart`, `/howto/search/8bit`, `/howto/search/watercolor` 등 임계값을 초과하는 수의 URL이 같은 prefix 아래 서로 다른 마지막 segment를 가질 때
- **THEN** 시스템은 이들을 동일 패턴으로 판별하고 템플릿 `/howto/search/{param}`을 생성한다

#### Scenario: mid-path parameterization 탐지 (경로 중간의 가변 segment)
- **WHEN** `/tags/TAG1/artwork`, `/tags/TAG2/artwork`, `/tags/TAG3/artwork` 등 임계값을 초과하는 수의 URL이 중간 segment만 다르고 나머지가 동일할 때
- **THEN** 시스템은 이들을 동일 패턴으로 판별하고 템플릿 `/tags/{param}/artwork`을 생성한다

#### Scenario: 다중 suffix를 가진 mid-path 탐지
- **WHEN** `/tags/TAG1/artwork`, `/tags/TAG1/illustrations`, `/tags/TAG2/artwork`, `/tags/TAG2/illustrations` 등 각 suffix별로 임계값을 초과할 때
- **THEN** 시스템은 suffix별로 별도 패턴을 생성한다: `/tags/{param}/artwork`, `/tags/{param}/illustrations`

#### Scenario: 깊은 중첩 mid-path 탐지
- **WHEN** `/users/USER1/posts/recent`, `/users/USER2/posts/recent` 등 임계값을 초과할 때
- **THEN** 시스템은 템플릿 `/users/{param}/posts/recent`을 생성한다

#### Scenario: 정적 리소스 경로는 머지하지 않음
- **WHEN** `/api/users/{id}`, `/api/posts/{id}` 등 서로 다른 리소스 경로가 존재하고, 가변 위치 이후의 segment가 모두 이미 parameterized(`{id}` 등)일 때
- **THEN** 시스템은 이들을 별도 경로로 유지한다

#### Scenario: depth가 다른 URL은 별도 처리
- **WHEN** `/tags/photo`(depth 2)와 `/tags/photo/artwork`(depth 3)가 존재할 때
- **THEN** depth가 다르므로 서로 다른 그룹에서 독립적으로 처리된다

#### Scenario: 임계값 미달 시 머지하지 않음
- **WHEN** 같은 패턴에 해당하는 URL 수가 임계값 이하일 때
- **THEN** 시스템은 해당 URL들을 개별 노드로 유지한다

---

### Requirement: 패턴 분석 결과를 기반으로 노드를 머지한다
패턴 분석으로 동일 패턴에 속하는 DB 노드들을 하나의 대표 노드로 통합해야 한다(SHALL). 대표 노드는 가장 먼저 생성된 노드여야 한다(SHALL).

#### Scenario: 패턴 내 노드 머지
- **WHEN** `/tags/TAG1/artwork`, `/tags/TAG2/artwork`, `/tags/TAG3/artwork`가 동일 패턴으로 판별될 때
- **THEN** 가장 먼저 생성된 노드를 대표로 선택하고, 나머지 노드의 엣지를 대표 노드로 재연결한 뒤, 나머지 노드를 삭제하고, 대표 노드의 URL을 `/tags/{param}/artwork`으로 변경한다

#### Scenario: 엣지 재연결 시 중복 제거
- **WHEN** 머지 대상 노드 A, B에 대해 X->A, X->B 엣지가 존재하고 B가 대표 노드일 때
- **THEN** X->A를 X->B로 재연결하면 중복이므로 X->A를 삭제한다

#### Scenario: self-loop 엣지 제거
- **WHEN** 머지 전 A->B 엣지가 있었고 A, B가 같은 대표 노드로 머지될 때
- **THEN** self-loop 엣지는 삭제된다

#### Scenario: 이미 머지된 사이트에 재실행
- **WHEN** 머지가 완료된 사이트에 대해 머지를 다시 실행할 때
- **THEN** 추가 변경 없이 완료된다 (멱등성)
