## ADDED Requirements

### Requirement: 태그·카테고리 라벨 시각 위계 분리

핀에 부착된 태그(tag) 또는 카테고리 라벨은 동일 카드/페이지의 본문·creator name·메타 정보·timestamp보다 한 단계 더 작은 글자 크기로 표시되어, 사용자가 태그를 콘텐츠 본체·메타 라인과 시각적으로 구분할 수 있어야 한다. 강조 수준은 디자인 가이드(DESIGN.md L26-35 typography scale)에 명시된 'tags, category labels' 카테고리 단계를 따른다.

#### Scenario: 핀 상세 페이지에 진입했을 때 태그 라벨이 creator name·timestamp보다 작게 표시된다

- **WHEN** 사용자가 임의의 핀 상세 페이지(`/pins/<id>`)에 진입하고 해당 핀에 태그가 1개 이상 부착되어 있다
- **THEN** 태그 라벨 텍스트는 같은 페이지의 creator name(DESIGN.md L33 creator name·meta 카테고리) 및 timestamp(DESIGN.md L34 timestamp·duration 카테고리)보다 작은 글자 크기로 렌더링되어, 'tags, category labels' 카테고리임이 시각적으로 식별된다

#### Scenario: 태그 위계가 모든 표시 사이트에 일관 적용된다

- **WHEN** 코드 SSoT에 DESIGN.md L35 'tags, category labels' 카테고리에 해당하는 텍스트(핀 태그·카테고리 칩 등)가 새로 추가되거나 기존 요소가 수정된다
- **THEN** 해당 요소는 디자인 가이드 'tags, category labels' 카테고리 크기를 적용받아 핀 목록(피드/검색)과 핀 상세 페이지 양쪽에서 동일한 시각 위계로 렌더링된다
