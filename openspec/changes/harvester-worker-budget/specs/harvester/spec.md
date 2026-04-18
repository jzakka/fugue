## ADDED Requirements

### Requirement: Harvester 워커는 100회 Dequeue 후 종료한다
Harvester 워커 프로세스는 `URLScheduler.Dequeue` 호출을 통해 처리할 URL을 정확히 100회 수령한 뒤 정상 종료(exit code 0)해야 한다(SHALL). 카운터는 "URL을 실제로 반환한 Dequeue 호출"만 증가시켜야 하며, 증가 시점은 **성공 Dequeue 직후**(= 호출이 URL을 리턴한 직후)여야 한다(SHALL). 빈 결과 또는 오류를 반환한 Dequeue 호출은 카운트하지 않아야 한다(SHALL NOT). budget 값(100)은 **빌드 타임 상수**로 고정되어야 하며(SHALL), 환경변수·설정 파일·CLI 플래그 등 어떤 런타임 수단으로도 변경 가능하게 노출되어서는 안 된다(SHALL NOT). 본 정책은 `pioneer-worker-budget`과 대칭이다.

#### Scenario: 100회 처리 후 정상 종료
- **WHEN** Harvester 워커가 시작되어 `URLScheduler.Dequeue`로부터 URL을 100회 수령하고 각 URL의 harvest 작업을 완료했을 때
- **THEN** 워커 프로세스는 exit code 0으로 종료한다.

#### Scenario: 빈 Dequeue는 카운트되지 않는다
- **WHEN** `URLScheduler.Dequeue`가 처리할 URL이 없어 빈 결과를 반환할 때
- **THEN** Dequeue 카운터는 증가하지 않으며, 워커는 짧게 대기한 뒤 다시 Dequeue를 시도한다.

#### Scenario: Dequeue 자체 오류는 카운트되지 않는다
- **WHEN** `URLScheduler.Dequeue` 호출이 (URL을 반환하지 않고) 오류를 반환할 때
- **THEN** Dequeue 카운터는 증가하지 않으며, 워커는 오류를 로깅한 뒤 다시 Dequeue를 시도한다.

#### Scenario: 카운터는 성공 Dequeue 직후 증가한다
- **WHEN** `URLScheduler.Dequeue`가 URL을 성공적으로 반환한 직후
- **THEN** Dequeue 카운터가 1 증가한 뒤에 해당 URL의 harvest 파이프라인이 시작된다.

#### Scenario: 99회까지는 종료하지 않는다
- **WHEN** Harvester 워커가 URL을 99회 수령하여 처리한 직후
- **THEN** 워커는 종료하지 않고 다음 Dequeue를 호출한다.

#### Scenario: budget은 빌드 시 상수
- **WHEN** 운영자가 환경변수·설정 파일·CLI 플래그로 budget 값을 변경하려 할 때
- **THEN** 워커 동작은 영향을 받지 않으며, 항상 100회 후 종료한다(budget은 빌드 타임에만 결정된다).

---

### Requirement: 진행 중 harvest 작업이 완료된 뒤에 종료한다
100회째 Dequeue로 받은 URL의 harvest 작업이 진행 중인 동안에는 워커가 종료해서는 안 된다(SHALL NOT). 특히 100회째 harvest 작업은 `harvester_frontier` 갱신 및 `harvester_frontier_pins` INSERT가 **단일 트랜잭션으로 커밋**된 시점까지 수행된 뒤에만 워커가 exit 0으로 종료해야 한다(SHALL). 작업이 실패로 끝나는 경우에도 frontier의 실패 기록(`harvest_error_count`/`next_harvest_at` 갱신)이 커밋된 뒤에 종료해야 한다(SHALL).

#### Scenario: 100회째 작업 완료 후 종료
- **WHEN** Harvester 워커가 100회째 Dequeue로 URL을 받아 harvest를 시작했고, `harvester_frontier` 갱신 및 `harvester_frontier_pins` INSERT 트랜잭션이 커밋되었을 때
- **THEN** 워커는 그 직후에 exit code 0으로 종료한다.

#### Scenario: 100회째 작업 진행 중에는 종료하지 않는다
- **WHEN** 100회째 Dequeue 직후 harvest 파이프라인이 외부 페이지 fetch·콘텐츠 추출·pin 생성·frontier 갱신 중 어느 단계든 수행 중일 때
- **THEN** 워커는 해당 단계가 완료되고 관련 DB 트랜잭션이 커밋될 때까지 종료하지 않는다.

#### Scenario: 100회째 작업이 실패해도 종료는 정상
- **WHEN** 100회째 URL의 harvest가 오류로 끝나고 frontier에 실패(`harvest_error_count++`, backoff 적용)가 기록된 직후
- **THEN** 워커는 exit code 0으로 종료한다(작업 실패가 워커 종료 코드를 바꾸지 않는다).

---

### Requirement: 워커 재시작은 supervisor의 책임이다
Harvester 워커 프로세스 자체는 자기 자신을 재기동하는 로직을 가져서는 안 된다(SHALL NOT). 종료 후 새 인스턴스를 띄우는 것은 외부 supervisor(systemd, k8s, docker restart policy, foreman 등)의 책임이어야 한다(SHALL). 종료 직전 워커는 종료 사유를 식별 가능한 로그(예: `reason=budget_exhausted`)를 남겨야 한다(SHALL).

#### Scenario: 워커는 자식을 spawn하지 않는다
- **WHEN** Harvester 워커가 100회 처리를 마치고 종료할 때
- **THEN** 워커는 새 워커 프로세스를 fork/exec하지 않고 단순히 종료한다.

#### Scenario: 종료 사유 로그
- **WHEN** Harvester 워커가 budget 소진으로 종료할 때
- **THEN** 종료 직전 Pioneer worker-budget과 동일한 필드(`reason=budget_exhausted`, `dequeues=100`)를 포함한 structured 로그 라인이 정확히 1회 출력된다(예: `msg="harvester worker: work budget exhausted" component=harvester_worker reason=budget_exhausted dequeues=100`).

#### Scenario: supervisor가 새 워커를 띄운다
- **WHEN** supervisor가 정상 종료(exit 0)한 Harvester 워커를 감지할 때
- **THEN** supervisor의 재시작 정책에 따라 새 워커 프로세스가 기동되며, 새 워커는 자체 카운터를 0부터 시작한다.

---

### Requirement: Dequeue 카운터는 워커 간 공유 상태가 아니다
Dequeue 카운터는 각 Harvester 워커 프로세스의 인메모리 변수로만 보관되어야 하며(SHALL), DB·Redis·frontier 등 워커 간 공유 저장소에 보관해서는 안 된다(SHALL NOT). 본 카운터는 워커 수명 관리용이며, 도메인 상태가 아니다.

#### Scenario: 복수 워커는 각자 독립 카운터를 갖는다
- **WHEN** Harvester 워커 두 인스턴스가 동시에 실행되고 있을 때
- **THEN** 한 워커가 50회를 처리해도 다른 워커의 카운터에는 영향을 주지 않는다.

#### Scenario: 카운터는 영속되지 않는다
- **WHEN** Harvester 워커가 종료된 직후
- **THEN** 카운터 값은 어디에도 저장되지 않으며, 새로 기동한 워커는 0에서 다시 시작한다.
