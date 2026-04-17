## ADDED Requirements

### Requirement: Pioneer 워커는 Dequeue 루프 100회 후 종료한다
Pioneer 워커 프로세스는 URLScheduler로부터의 Dequeue 루프를 **최대 100회 반복**한 뒤 정상 종료해야 한다(SHALL). 반복 횟수 카운터는 프로세스 로컬 인메모리 값이며, Dequeue 호출 성공 여부(URL을 받았는지, 빈 응답인지)와 무관하게 **루프 이터레이션 1회마다 1씩 증가**해야 한다(SHALL).

#### Scenario: 100회 Dequeue 루프 후 종료
- **WHEN** Pioneer 워커가 Dequeue 루프를 100회 반복했을 때
- **THEN** 워커는 더 이상 새 URL을 Dequeue하지 않고 프로세스를 종료한다

#### Scenario: 정상 종료 코드 반환
- **WHEN** 워커가 Dequeue 루프 100회 상한에 도달하여 종료할 때
- **THEN** 프로세스는 exit code 0으로 종료한다

#### Scenario: 빈 Dequeue 응답도 카운트에 포함
- **WHEN** Dequeue 호출이 URL을 반환하지 않는 idle 상태에서도 루프를 반복할 때
- **THEN** 각 이터레이션은 예산 카운터를 1씩 증가시킨다

#### Scenario: 카운터는 프로세스 로컬
- **WHEN** 여러 Pioneer 프로세스가 동시에 실행될 때
- **THEN** 각 프로세스는 독립적으로 자신의 100회 예산을 소비하며 카운터를 공유하지 않는다

#### Scenario: 100회 미만에서는 계속 루프 수행
- **WHEN** 워커의 누적 Dequeue 루프 반복 횟수가 100 미만일 때
- **THEN** 워커는 계속 다음 Dequeue를 수행한다

---

### Requirement: 진행 중 URL은 완료 후 마무리한다
예산 소진 검사는 **Dequeue 직전(루프 상단)** 에서 수행해야 하며(SHALL), 이미 Dequeue하여 처리 중인 URL은 fetch/링크 추출/enqueue 사이클을 끝까지 수행한 뒤 루프를 빠져나와야 한다(SHALL). 진행 중 작업을 중간에 버리고 종료해서는 안 된다(SHALL NOT).

#### Scenario: 진행 중 URL 처리 완료 후 종료
- **WHEN** 99번째 이터레이션에서 Dequeue한 URL을 처리 중이고, 처리 완료 시점에 카운터가 100에 도달했을 때
- **THEN** 해당 URL의 fetch/extract/enqueue를 모두 끝낸 뒤 워커가 종료한다

#### Scenario: mid-flight 중단 금지
- **WHEN** 카운터가 100에 도달해도 현재 처리 중인 URL이 아직 진행 중일 때
- **THEN** 워커는 fetch, 링크 추출, 링크 enqueue를 완료할 때까지 종료를 지연한다

#### Scenario: 100회째 완료 후 추가 Dequeue 금지
- **WHEN** 100번째 이터레이션의 URL 처리가 완료되었을 때
- **THEN** 워커는 새 Dequeue를 시도하지 않고 즉시 종료 절차에 진입한다

---

### Requirement: 워커 재시작은 supervisor 책임이다
Pioneer 워커는 예산 소진 시 단순히 종료할 뿐이며(SHALL), 자신을 재기동하지 않아야 한다(SHALL NOT). 재시작은 외부 supervisor(systemd, Kubernetes Deployment, Docker restart policy 등)의 책임이며 본 스펙의 범위 밖이다.

#### Scenario: 워커는 자체 재시작하지 않음
- **WHEN** 워커가 예산 소진으로 종료할 때
- **THEN** 워커는 새 워커 프로세스를 스폰하거나 내부 루프를 재개하지 않는다

#### Scenario: 종료 시 상태 청산
- **WHEN** 워커가 예산 소진으로 종료할 때
- **THEN** 인메모리 visited 맵·큐·기타 세션 상태는 프로세스 종료와 함께 폐기되며 외부로 전달되지 않는다

#### Scenario: 종료 사유 로그
- **WHEN** 워커가 예산 소진으로 종료하기 직전일 때
- **THEN** 워커는 "work budget exhausted" 취지의 로그를 남겨 재시작 사이클을 관측 가능하게 한다
