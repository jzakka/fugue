## ADDED Requirements

### Requirement: Pioneer 실행 생명주기를 추적한다
시스템은 각 Pioneer 실행의 시작 시각, 완료 시각, 상태를 기록해야 한다.

#### Scenario: 실행 시작 기록
- **WHEN** Pioneer가 실행을 시작할 때
- **THEN** 실행 중 상태와 시작 시각으로 실행 레코드를 생성한다

#### Scenario: 실행 완료 기록
- **WHEN** Pioneer가 성공적으로 완료될 때
- **THEN** 상태를 완료로 갱신하고 완료 시각을 기록한다

#### Scenario: 실행 실패 기록
- **WHEN** Pioneer가 치명적 오류를 만났을 때
- **THEN** 상태를 실패로 갱신하고 오류 메시지를 기록한다

### Requirement: Pioneer 탐색 통계를 추적한다
시스템은 발견한 노드, 갱신한 노드, 생성/재사용한 스크립트 수를 세어야 한다.

#### Scenario: 탐색 카운터 증가
- **WHEN** Pioneer가 실행 중 노드를 발견하거나 스크립트를 생성/재사용할 때
- **THEN** 해당하는 카운터를 증가시킨다

### Requirement: Pioneer AI 비용을 추적한다
시스템은 모든 AI API 호출과 비용을 실행별로 합산해야 한다.

#### Scenario: AI 비용 누적
- **WHEN** Pioneer가 AI API를 호출할 때마다
- **THEN** 호출 횟수와 비용을 누적하여 기록한다

### Requirement: Harvester 실행 생명주기를 추적한다
시스템은 각 Harvester 실행의 시작, 완료, 상태를 기록해야 한다.

#### Scenario: Harvester 시작 기록
- **WHEN** Harvester가 시작될 때
- **THEN** 실행 중 상태와 시작 시각으로 실행 레코드를 생성한다

### Requirement: Harvester 추출 통계를 추적한다
시스템은 방문한 노드(성공/실패), 추출한 아이템, 중복 제거, 생성한 핀 수를 세어야 한다.

#### Scenario: 추출 메트릭 기록
- **WHEN** Harvester 실행이 완료될 때
- **THEN** 방문 노드 수, 성공/실패 수, 추출 아이템 수, 중복 제거 수, 생성 핀 수를 기록한다

### Requirement: 이력 분석을 지원한다
시스템은 추세 분석과 디버깅을 위해 모든 실행 기록을 보존해야 한다.

#### Scenario: 실행 이력 조회
- **WHEN** 특정 사이트의 최근 30일 실행 이력을 조회할 때
- **THEN** 모든 실행의 타임스탬프, 통계, 오류를 반환한다
