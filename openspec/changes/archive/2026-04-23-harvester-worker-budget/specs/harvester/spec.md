## ADDED Requirements

### Requirement: Harvester 워커는 100회 Dequeue 후 종료한다
Harvester 워커 프로세스는 `URLScheduler.Dequeue` 호출을 통해 처리할 URL을 정확히 100회 수령한 뒤 정상 종료(exit code 0)해야 한다(SHALL). 카운터는 "URL을 실제로 반환한 Dequeue 호출"만 증가시켜야 하며, 증가 시점은 **성공 Dequeue 직후**(= 호출이 URL을 리턴한 직후)여야 한다(SHALL). 빈 결과 또는 오류를 반환한 Dequeue 호출은 카운트하지 않아야 한다(SHALL NOT). budget 값(100)은 **빌드 타임 상수**로 고정되어야 하며(SHALL), 환경변수·설정 파일·CLI 플래그 등 어떤 런타임 수단으로도 변경 가능하게 노출되어서는 안 된다(SHALL NOT). ctx 취소 등 외부 종료 신호로 워커가 100회 미만에서 종료되는 경로는 본 정책과 독립적이며, budget은 상한(상향 제한)으로 기능한다. 본 정책은 `pioneer-worker-budget`과 대칭이다.

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

#### Scenario: ctx 취소 경로는 budget과 독립적이다
- **WHEN** 100회 도달 전에 외부 ctx 취소 또는 SIGTERM으로 워커가 종료 신호를 받았을 때
- **THEN** 워커는 budget 미소진 상태에서도 종료할 수 있으며(진행 중 fetch/pipeline은 ctx 전파로 중단될 수 있음), budget 정책은 위반되지 않는다(budget은 상향 제한이지 하한이 아니다).

---

### Requirement: 진행 중 harvest 작업이 완료된 뒤에 종료한다
100회째 Dequeue로 받은 URL의 harvest 작업이 진행 중인 동안에는 워커가 종료해서는 안 된다(SHALL NOT). 특히 100회째 URL 처리의 **최종 상태 전이**가 완료된 뒤에만 워커가 exit 0으로 종료해야 한다(SHALL). 최종 상태 전이는 다음 두 경로 중 하나를 의미한다:

- **성공 경로**: `SetStatus(harvested, pinIDs)` 호출이 성공적으로 반환(= `harvester_frontier` 갱신 및 `harvester_frontier_pins` INSERT가 단일 트랜잭션으로 커밋)된 직후.
- **실패 경로**: 기존 "Harvester 실패 시 SetStatus + RecordHarvestError를 둘 다 호출한다" requirement에 따라 `SetStatus(harvest_failed, nil)` 호출과 `RecordHarvestError(errorKind)` 호출이 **둘 다 완료된** 직후.

작업 실패가 워커 종료 코드를 바꾸지 않는다(SHALL). 실패 경로에서도 exit code는 0이다.

#### Scenario: 100회째 작업 성공 완료 후 종료
- **WHEN** Harvester 워커가 100회째 Dequeue로 URL을 받아 harvest를 시작했고, `SetStatus(harvested, pinIDs)` 호출이 성공적으로 반환되었을 때
- **THEN** 워커는 그 직후에 exit code 0으로 종료한다.

#### Scenario: 100회째 작업 진행 중에는 종료하지 않는다
- **WHEN** 100회째 Dequeue 직후 harvest 파이프라인이 외부 페이지 fetch·콘텐츠 추출·pin 생성·상태 전이 중 어느 단계든 수행 중일 때
- **THEN** 워커는 해당 단계가 완료되고 최종 상태 전이 호출이 반환될 때까지 종료하지 않는다.

#### Scenario: 100회째 작업이 실패해도 종료는 정상
- **WHEN** 100회째 URL의 fetch/parse/pin 생성이 오류로 끝나고 `SetStatus(harvest_failed, nil)` 호출과 `RecordHarvestError(errorKind)` 호출이 **둘 다 완료**된 직후
- **THEN** 워커는 exit code 0으로 종료한다(작업 실패가 워커 종료 코드를 바꾸지 않는다).

#### Scenario: 실패 경로 dual-call 도중 종료하지 않는다
- **WHEN** 100회째 URL이 실패로 끝나고 `SetStatus(harvest_failed, nil)`는 반환되었으나 `RecordHarvestError`가 아직 호출되지 않은 상태일 때
- **THEN** 워커는 `RecordHarvestError`가 반환될 때까지 종료를 지연한다(둘 중 하나만 호출하고 종료하지 않는다).

---

### Requirement: 워커 재시작은 supervisor의 책임이다
Harvester 워커 프로세스 자체는 자기 자신을 재기동하는 로직을 가져서는 안 된다(SHALL NOT). 종료 후 새 인스턴스를 띄우는 것은 외부 supervisor(systemd, Kubernetes Deployment, Docker restart policy, foreman 등)의 책임이어야 한다(SHALL). 종료 직전 워커는 `pioneer-worker-budget`과 동일한 필드(`reason=budget_exhausted`, `dequeues=100`, `component=harvester_worker`)를 포함한 **기계 파싱 가능한 key=value 포맷 로그**를 정확히 1회 남겨야 한다(SHALL). (구체 로그 문자열 예시는 tasks.md §2.1 참조.)

#### Scenario: 워커는 자식을 spawn하지 않는다
- **WHEN** Harvester 워커가 100회 처리를 마치고 종료할 때
- **THEN** 워커는 새 워커 프로세스를 fork/exec하거나 내부 루프를 재개하지 않고 단순히 종료한다.

#### Scenario: 종료 사유 로그
- **WHEN** Harvester 워커가 budget 소진으로 종료하기 직전일 때
- **THEN** Pioneer worker-budget과 동일한 필드(`reason=budget_exhausted`, `dequeues=100`, `component=harvester_worker`)를 포함한 key=value 포맷 로그 라인이 정확히 1회 출력된다.

#### Scenario: supervisor가 새 워커를 띄운다
- **WHEN** supervisor(예: docker restart policy, systemd `Restart=always`, k8s `restartPolicy: Always`)가 정상 종료(exit 0)한 Harvester 워커를 감지할 때
- **THEN** supervisor의 재시작 정책에 따라 새 워커 프로세스가 기동되며, 새 워커는 자체 카운터를 0부터 시작한다.

#### Scenario: 종료 시 상태 청산
- **WHEN** 워커가 budget 소진으로 종료할 때
- **THEN** 인메모리 Dequeue 카운터·Goja 런타임·HTTP 커넥션·임시 파일 등 세션 상태는 프로세스 종료와 함께 폐기되며 외부로 전달되지 않는다.

---

### Requirement: Dequeue 카운터는 워커 간 공유 상태가 아니다
Dequeue 카운터는 각 Harvester 워커 프로세스의 인메모리 변수로만 보관되어야 하며(SHALL), DB·Redis·frontier 등 워커 간 공유 저장소에 보관해서는 안 된다(SHALL NOT). 본 카운터는 워커 수명 관리용이며, 도메인 상태가 아니다.

#### Scenario: 복수 워커는 각자 독립 카운터를 갖는다
- **WHEN** Harvester 워커 두 인스턴스가 동시에 실행되고 있을 때
- **THEN** 한 워커가 50회를 처리해도 다른 워커의 카운터에는 영향을 주지 않는다.

#### Scenario: 카운터는 영속되지 않는다
- **WHEN** Harvester 워커가 종료된 직후
- **THEN** 카운터 값은 어디에도 저장되지 않으며, 새로 기동한 워커는 0에서 다시 시작한다.

---

## MODIFIED Requirements

### Requirement: 메인 루프는 snapshot-first fetch → PinDocument → Pin 생성 → SetStatus 순서를 따른다
Harvester의 단일 iteration은 다음 단계를 순서대로 수행해야 한다(SHALL):
1. `scheduler.Dequeue(scheduler.QueueHarvester)`로 처리 대상 URL을 claim한다.
2. `harvester-snapshot-first-fetch` capability가 제공하는 snapshot-first 경로로 HTML을 획득한다(snapshot_key가 있으면 snapshot 우선, miss 시 HTTP live fetch).
3. `harvester-pin-document` capability의 `harvestPipeline.Process`로 HTML을 `PinDocument`로 파싱한다.
4. `PinDocument.Pinnable`이 true이면 Pin을 생성하여 `pinIDs []uuid.UUID`를 수집한다. false이면 Pin 생성을 건너뛴다.
5. 성공 시 `scheduler.SetStatus(url, "harvested", pinIDs)`를 호출한다. `pinIDs`가 nil 또는 빈 슬라이스이면 매핑 없이 완료 표기한다.

각 단계의 실패는 다음 단계 실행을 중단해야 하며(SHALL), 실패 처리(본 spec의 별도 requirement)를 따라야 한다.

**워커 종료 조건**(work budget 소진 = 성공 Dequeue 100회 후 종료)은 본 capability의 "Harvester 워커는 100회 Dequeue 후 종료한다" requirement에 정의되어 있다. 본 루프 requirement는 단일 iteration의 **단계 순서**만 규범화한다.

#### Scenario: 정상 흐름 - Pin 1건 생성
- **WHEN** `Dequeue`가 URL `U`를 반환하고, snapshot-first fetch와 PinDocument 파싱이 성공하고 `Pinnable = true`이며 Pin 1건이 생성될 때
- **THEN** Harvester는 `scheduler.SetStatus(U, "harvested", []uuid.UUID{pinID})`를 호출한 뒤 다음 iteration을 시작한다.

#### Scenario: 정상 흐름 - Pin N건 생성
- **WHEN** `PinDocument`가 복수 Pin으로 materialize되어 `pinIDs`가 길이 N(N>=2)인 슬라이스일 때
- **THEN** Harvester는 `scheduler.SetStatus(U, "harvested", pinIDs)`를 **단일 호출**로 전달하고, scheduler 구현이 `harvested_at` UPDATE와 `harvester_frontier_pins` 일괄 INSERT를 한 트랜잭션에서 처리한다.

#### Scenario: Pinnable = false 시 Pin 생성 스킵
- **WHEN** fetch와 파싱은 성공했으나 `PinDocument.Pinnable == false`일 때
- **THEN** Harvester는 Pin을 생성하지 않고 `scheduler.SetStatus(U, "harvested", nil)`을 호출하여 해당 row를 완료 상태로 표기한다. `harvester_frontier_pins`에는 아무 row도 INSERT되지 않는다.

#### Scenario: 빈 pinIDs 슬라이스도 완료 표기로 처리
- **WHEN** `pinIDs`가 nil이거나 길이 0인 슬라이스일 때
- **THEN** `SetStatus(U, "harvested", nil)` 호출과 동일하게 처리되어 매핑 없이 `harvested_at`만 갱신된다.

#### Scenario: 루프는 상기 단계를 반복하되 budget 요건에 따라 종료한다
- **WHEN** Harvester 프로세스가 정상 구동 중일 때
- **THEN** 메인 루프는 상기 단계를 반복하며, 워커 종료 조건(work budget 소진)은 본 capability의 "Harvester 워커는 100회 Dequeue 후 종료한다" requirement에 정의되어 있다(이 루프 스펙 내에서는 단계 순서만 규범화한다).
