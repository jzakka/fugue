## ADDED Requirements

### Requirement: AI CLI 클라이언트가 비인터랙티브 모드로 동작한다
CLI 클라이언트가 `codex` 명령을 사용할 때, 비인터랙티브 서브커맨드를 자동으로 적용하여 TTY 없이 stdin으로 프롬프트를 전달할 수 있어야 한다(SHALL).

#### Scenario: codex 명령에 비인터랙티브 모드 자동 적용
- **WHEN** CLI 클라이언트의 command가 "codex"이고 Call을 실행할 때
- **THEN** 명령이 비인터랙티브 모드로 실행되며 stdin에서 프롬프트를 읽는다

#### Scenario: codex가 아닌 커스텀 명령에는 모드를 변경하지 않음
- **WHEN** CLI 클라이언트의 command가 "codex"가 아닌 다른 값일 때
- **THEN** 명령 args를 변경하지 않고 그대로 실행한다

#### Scenario: 기존 args가 있을 때 비인터랙티브 모드 인자가 우선 적용
- **WHEN** CLI 클라이언트의 command가 "codex"이고 추가 args가 설정되어 있을 때
- **THEN** 비인터랙티브 모드 인자가 기존 인자보다 우선 적용된다

---

### Requirement: show-map 시각화가 저장된 스크립트 기반으로 구현 상태를 판정한다
그래프 시각화에서 노드의 스크립트 구현 상태(HasScript)는 저장된 스크립트 데이터를 기반으로 판정해야 한다(SHALL). 파일 시스템을 조회하지 않아야 한다(SHALL).

#### Scenario: 스크립트가 존재하는 노드의 HasScript 판정
- **WHEN** 해당 사이트와 노드 타입 조합에 대한 스크립트가 저장소에 존재할 때
- **THEN** 해당 노드의 HasScript가 true로 설정된다

#### Scenario: 스크립트가 없는 노드의 HasScript 판정
- **WHEN** 해당 사이트와 노드 타입 조합에 대한 스크립트가 저장소에 존재하지 않을 때
- **THEN** 해당 노드의 HasScript가 false로 설정된다
