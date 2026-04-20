## ADDED Requirements

### Requirement: Pioneer 워커는 성공 Dequeue 100회 후 종료한다
Pioneer 워커 프로세스는 `URLScheduler.Dequeue` 호출을 통해 URL을 최대 100회까지 수령하여 각 URL의 처리 사이클을 완료한 뒤 정상 종료(exit code 0)해야 한다(SHALL). 카운터는 "URL을 실제로 반환한 Dequeue 호출"만 증가시켜야 하며(SHALL), 증가 시점은 **성공 Dequeue 직후**(= 호출이 URL을 리턴한 직후)여야 한다(SHALL). 빈 결과 또는 오류를 반환한 Dequeue 호출은 카운트하지 않아야 한다(SHALL NOT). budget 값(100)은 **빌드 타임 상수**로 고정되어야 하며(SHALL), 환경변수·설정 파일·CLI 플래그 등 어떤 런타임 수단으로도 변경 가능하게 노출되어서는 안 된다(SHALL NOT). ctx 취소 등 외부 종료 신호로 워커가 100회 미만에서 종료되는 경로는 본 정책과 독립적이며, budget은 상한(상향 제한)으로 기능한다. 본 정책은 `harvester-worker-budget`과 대칭이다.

#### Scenario: 99회까지는 종료하지 않는다
- **WHEN** Pioneer 워커가 `URLScheduler.Dequeue`로부터 URL을 99회 수령하여 각 URL의 fetch/링크 추출/Enqueue/SetStatus 사이클을 모두 완료한 직후
- **THEN** 워커는 종료하지 않고 다음 Dequeue를 호출한다.

#### Scenario: 100회째 Dequeue로 받은 URL 처리 완료 후 exit 0
- **WHEN** Pioneer 워커가 100회째 Dequeue로 URL을 수령하여 fetch → 링크 추출 → Enqueue(신규 링크) → SetStatus(frontier 갱신)까지 모두 완료했을 때
- **THEN** 워커 프로세스는 추가 Dequeue를 시도하지 않고 exit code 0으로 종료한다.

#### Scenario: ctx 취소 경로는 budget과 독립적이다
- **WHEN** 100회에 도달하기 전에 외부 ctx 취소 또는 SIGTERM으로 워커가 종료 신호를 받았을 때
- **THEN** 워커는 budget 미소진 상태에서도 종료할 수 있으며(진행 중 fetch는 ctx 전파로 중단될 수 있음), budget 정책은 위반되지 않는다 (budget은 상향 제한이지 하한이 아니다).

#### Scenario: 빈 Dequeue는 카운트되지 않는다
- **WHEN** `URLScheduler.Dequeue`가 URL을 반환하지 않을 때
- **THEN** Dequeue 카운터는 증가하지 않으며, 워커는 다음 Dequeue를 시도한다.

#### Scenario: Dequeue 자체 오류는 카운트되지 않는다
- **WHEN** `URLScheduler.Dequeue` 호출이 (URL을 반환하지 않고) 오류를 반환할 때
- **THEN** Dequeue 카운터는 증가하지 않으며, 워커는 오류를 로깅한 뒤 다시 Dequeue를 시도한다.

#### Scenario: 카운터는 성공 Dequeue 직후 증가한다
- **WHEN** `URLScheduler.Dequeue`가 URL을 성공적으로 반환한 직후
- **THEN** Dequeue 카운터가 1 증가한 뒤에 해당 URL의 fetch 파이프라인이 시작된다.

#### Scenario: budget은 빌드 시 상수
- **WHEN** 운영자가 환경변수나 설정 파일, CLI 플래그로 budget 값을 변경하려 할 때
- **THEN** 워커 동작은 영향을 받지 않으며, 항상 성공 Dequeue 100회 후 종료한다.

---

### Requirement: 진행 중 URL 처리는 중단 없이 완료한다
budget 종료 판정(루프 break 결정)은 **현재 URL 처리 사이클이 완료된 뒤**에만 수행해야 한다(SHALL). 카운터 증가 시점은 별도 Requirement("카운터는 성공 Dequeue 직후 증가한다")가 정의한다. 100회째 Dequeue로 받은 URL의 fetch, 링크 추출, Enqueue(신규 링크 재투입), SetStatus(frontier 갱신)가 모두 끝날 때까지 워커는 종료를 지연해야 한다(SHALL). 진행 중 작업을 중간에 버리고 종료해서는 안 된다(SHALL NOT).

#### Scenario: 100회째 작업이 끝날 때까지 종료 지연
- **WHEN** 카운터가 100에 도달한 뒤에도 현재 URL의 fetch 또는 링크 추출 또는 Enqueue 또는 SetStatus가 아직 진행 중일 때
- **THEN** 워커는 모든 단계를 완료할 때까지 종료하지 않는다.

#### Scenario: 100회째 처리 실패도 정상 종료
- **WHEN** 100회째 URL의 fetch가 오류로 끝나고 frontier에 실패가 기록된 직후
- **THEN** 워커는 exit code 0으로 종료한다 (작업 실패가 워커 종료 코드를 바꾸지 않는다).

#### Scenario: 100회째 완료 후 추가 Dequeue 금지
- **WHEN** 100회째 URL 처리가 완료되었을 때
- **THEN** 워커는 새 Dequeue를 시도하지 않고 즉시 종료 절차에 진입한다.

---

### Requirement: 워커 재시작은 supervisor의 책임이다
Pioneer 워커 프로세스 자체는 자기 자신을 재기동하는 로직을 가져서는 안 된다(SHALL NOT). 종료 후 새 인스턴스를 띄우는 것은 외부 supervisor(systemd, Kubernetes Deployment, Docker restart policy 등)의 책임이어야 한다(SHALL). 종료 직전 워커는 Harvester 워커와 동일한 필드(`reason=budget_exhausted`, `dequeues=100`)를 포함한 기계 파싱 가능한 key=value 포맷 로그(예: `msg="pioneer worker: work budget exhausted" component=pioneer_worker reason=budget_exhausted dequeues=100`)를 정확히 1회 남겨야 한다(SHALL).

#### Scenario: 워커는 자식을 spawn하지 않는다
- **WHEN** Pioneer 워커가 100회 처리를 마치고 종료할 때
- **THEN** 워커는 새 워커 프로세스를 fork/exec하거나 내부 루프를 재개하지 않고 단순히 종료한다.

#### Scenario: 종료 사유 로그
- **WHEN** Pioneer 워커가 budget 소진으로 종료하기 직전일 때
- **THEN** Harvester worker-budget과 동일한 필드(`reason=budget_exhausted`, `dequeues=100`)를 포함한 key=value 포맷 로그 라인(예: `msg="pioneer worker: work budget exhausted" component=pioneer_worker reason=budget_exhausted dequeues=100`)이 정확히 1회 출력된다.

#### Scenario: supervisor가 새 워커를 띄운다
- **WHEN** supervisor(예: docker restart policy, systemd `Restart=always`, k8s `restartPolicy: Always`)가 exit 0로 종료된 Pioneer 워커를 감지할 때
- **THEN** supervisor의 재시작 정책에 따라 새 워커 프로세스가 기동되며, 새 워커는 자체 카운터를 0부터 시작한다.

#### Scenario: 종료 시 상태 청산
- **WHEN** 워커가 budget 소진으로 종료할 때
- **THEN** 인메모리 visited 맵·큐·기타 세션 상태는 프로세스 종료와 함께 폐기되며 외부로 전달되지 않는다.

---

### Requirement: Dequeue 카운터는 워커 간 공유 상태가 아니다
Dequeue 카운터는 각 Pioneer 워커 프로세스의 인메모리 변수로만 보관되어야 하며(SHALL), DB·Redis·frontier 등 워커 간 공유 저장소에 보관해서는 안 된다(SHALL NOT). 본 카운터는 워커 수명 관리용이며, 도메인 상태가 아니다.

#### Scenario: 복수 워커는 각자 독립 카운터를 갖는다
- **WHEN** Pioneer 워커 두 인스턴스가 동시에 실행되고 있을 때
- **THEN** 한 워커가 50회를 처리해도 다른 워커의 카운터에는 영향을 주지 않는다.

#### Scenario: 카운터는 영속되지 않는다
- **WHEN** Pioneer 워커가 종료된 직후
- **THEN** 카운터 값은 어디에도 저장되지 않으며, 새로 기동한 워커는 0에서 다시 시작한다.

---

## MODIFIED Requirements

### Requirement: Pioneer 메인 루프는 Dequeue → fetch → snapshot → parse → filter → Enqueue(pioneer) + EnqueueHarvester → SetStatus 반복이다
Pioneer의 메인 루프는 `scheduler.Dequeue(QueuePioneer)`, URL fetch, snapshot 저장, 링크 추출, `FilterChain.Apply`, `scheduler.Enqueue(QueuePioneer, filteredURLs)`, `scheduler.EnqueueHarvester(url, snapshotKey)`, `scheduler.SetStatus(url, "fetched", nil)`의 반복으로 구성되어야 한다(SHALL). 추가 단계를 이 루프의 책임으로 포함하지 않아야 한다(SHALL NOT).

#### Scenario: 정상 경로의 루프 순서
- **WHEN** Pioneer가 한 번의 성공 반복을 수행할 때
- **THEN** 순서대로 `scheduler.Dequeue(QueuePioneer)` → URL fetch → snapshot 저장(`snapshot_key` 획득) → 링크 추출 → `FilterChain.Apply` → `scheduler.Enqueue(QueuePioneer, filteredURLs)` → `scheduler.EnqueueHarvester(url, snapshotKey)` → `scheduler.SetStatus(url, "fetched", nil)` 이 실행된다.

#### Scenario: FilterChain은 Enqueue 직전에 Pioneer consumer가 호출한다
- **WHEN** Pioneer가 링크 목록을 `pioneer_frontier`에 넣기 직전일 때
- **THEN** Pioneer consumer가 `filterChain.Apply(links)`를 호출하여 필터를 통과한 링크만 `scheduler.Enqueue(QueuePioneer, ...)`로 투입한다 (필터 구성 자체는 `pioneer-link-filter-policy`가 정의하며, Pioneer는 호출 타이밍만 책임진다).

#### Scenario: 추출한 링크를 같은 pioneer_frontier로 다시 Enqueue한다
- **WHEN** 한 URL의 fetch 결과에서 n개 링크를 추출하여 필터를 통과시켰을 때
- **THEN** Pioneer는 `scheduler.Enqueue(scheduler.QueuePioneer, filteredURLs)`를 호출하여 동일 scheduler의 `pioneer_frontier`에 다시 투입한다 (별도 큐/채널/파일로 내보내지 않는다).

#### Scenario: 루프에 별도 sleep/backoff를 두지 않는다
- **WHEN** Pioneer consumer 루프 코드를 정적 분석할 때
- **THEN** 빈 큐 폴링이나 실패 재시도를 위한 `time.Sleep`/`time.After` 호출이 루프 본문에 존재하지 않는다 (폴링 책임은 scheduler 내부에 있다).

#### Scenario: 루프는 상기 단계를 반복한다
- **WHEN** Pioneer 프로세스가 정상 구동 중일 때
- **THEN** 메인 루프는 상기 단계를 반복하며, 워커 종료 조건(work budget 소진)은 본 capability의 "Pioneer 워커는 성공 Dequeue 100회 후 종료한다" requirement에 정의되어 있다 (이 루프 스펙 내에서는 단계 순서만 규범화한다).
