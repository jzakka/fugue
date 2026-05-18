## ADDED Requirements

### Requirement: Timestamp 데이터 위계 분리

가입일·생성일 등 시간 데이터(timestamp)는 동일 카드/페이지의 본문·라벨·메타 정보보다 한 단계 작은 글자 크기로 표시되어, 사용자가 timestamp을 콘텐츠 본체와 시각적으로 구분할 수 있어야 한다. 강조 수준은 디자인 가이드(DESIGN.md L26-35 typography scale)에 명시된 카테고리 단계를 따른다.

#### Scenario: 크리에이터 카드에 진입했을 때 가입일이 닉네임·메타보다 작게 표시된다

- **WHEN** 사용자가 크리에이터 검색 결과 또는 크리에이터 카드가 포함된 페이지에 진입한다
- **THEN** 가입일(timestamp) 텍스트는 닉네임·메타 정보(DESIGN.md typography scale의 secondary text / creator name·meta 카테고리)보다 작은 글자 크기로 렌더링되어 timestamp 카테고리임이 시각적으로 식별된다

#### Scenario: timestamp 위계가 모든 표시 사이트에 일관 적용된다

- **WHEN** 코드 SSoT에 DESIGN.md L34 'timestamps + duration' 카테고리에 해당하는 텍스트(가입일·생성일·duration 등)가 새로 추가되거나 기존 요소가 수정된다
- **THEN** 해당 요소는 디자인 가이드 timestamp 카테고리 크기를 적용받아 다른 timestamp 표시와 동일한 위계로 렌더링된다
