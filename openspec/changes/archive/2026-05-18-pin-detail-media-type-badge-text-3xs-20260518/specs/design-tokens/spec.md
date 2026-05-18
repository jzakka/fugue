## ADDED Requirements

### Requirement: 미디어 카테고리 라벨 시각 위계 분리

핀에 부착된 미디어 카테고리 라벨(작품의 미디어 타입을 분류해 사용자에게 노출하는 라벨)은 동일 카드/페이지의 본문·creator name·메타 정보·timestamp보다 한 단계 더 작은 글자 크기로 표시되어, 사용자가 미디어 카테고리 라벨을 콘텐츠 본체·메타 라인과 시각적으로 구분할 수 있어야 한다. 강조 수준은 디자인 가이드(DESIGN.md L26-35 typography scale)에 명시된 'tags, category labels' 카테고리 단계를 따른다.

#### Scenario: 핀 상세 페이지에 진입했을 때 미디어 카테고리 라벨이 creator name·timestamp보다 작게 표시된다

- **WHEN** 사용자가 임의의 핀 상세 페이지(`/pins/<id>`)에 진입하고 해당 핀에 미디어 카테고리가 부여되어 있다
- **THEN** 미디어 카테고리 라벨 텍스트는 같은 페이지의 creator name(DESIGN.md L33 creator name·meta 카테고리) 및 timestamp(DESIGN.md L34 timestamp·duration 카테고리)보다 작은 글자 크기로 렌더링되어, 'tags, category labels' 카테고리임이 시각적으로 식별된다

#### Scenario: 미디어 카테고리 라벨과 태그 라벨이 동일 위계로 정렬된다

- **WHEN** 핀 상세 페이지에서 미디어 카테고리 라벨과 태그 라벨이 동시에 표시된다
- **THEN** 두 라벨은 같은 'tags, category labels' 카테고리에 속하므로 동일한 글자 크기 단계로 렌더링되어 시각 위계가 일관된다
