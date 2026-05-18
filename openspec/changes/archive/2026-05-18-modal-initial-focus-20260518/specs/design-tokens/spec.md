## ADDED Requirements

### Requirement: 모달 dialog가 열린 직후 키보드 포커스는 dialog 내부에 위치한다

플랫폼의 모달 dialog 컴포넌트는 열린 직후 키보드 포커스가 dialog container 또는 dialog 내부 actionable 요소로 이동해야 한다(SHALL). 포커스는 modal 바깥 요소에 머물러서는 안 된다(MUST NOT). dialog container 자체에 포커스를 두는 경우 container는 프로그래밍 방식 focus를 받을 수 있도록 표시되어야 한다(SHALL).

#### Scenario: 모달이 열리면 dialog 내부로 포커스가 이동한다

- **WHEN** 사용자가 트리거 버튼을 눌러 모달 dialog를 열 때
- **THEN** 키보드 포커스가 dialog container 또는 dialog 내부 actionable 요소로 이동한다
- **AND** 모달 열리기 전 트리거 버튼에 있던 포커스는 dialog 바깥에 머무르지 않는다

#### Scenario: dialog container에 프로그래밍 방식 포커스가 가능하다

- **WHEN** dialog container 자체에 포커스를 두는 모달일 때
- **THEN** dialog container는 자연스러운 Tab 순회에 포함되지 않으면서도 프로그래밍 방식 `.focus()` 호출을 받을 수 있다

#### Scenario: 모달 내부의 조건부 인풋 자동 포커스 패턴은 보존된다

- **WHEN** 모달 내부에서 조건부로 펼쳐지는 폼(예: 인라인 생성 form)의 인풋이 펼침 시점에 자동 포커스를 받도록 정착된 컴포넌트일 때
- **THEN** modal 열림 시점의 dialog 포커스와 조건부 인풋의 펼침 시점 포커스는 시간상 분리되어 서로 간섭하지 않는다
- **AND** 모달 mount 시점에는 dialog가 포커스를 받고, 조건부 폼 펼침 시점에는 해당 인풋이 포커스를 받는다
