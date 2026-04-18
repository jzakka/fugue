## Why

Harvester 워커 프로세스가 무기한 실행되면 Goja VM 누적 메모리, FD 누수, harvest 스크립트의 잠재적 상태 잔존 등으로 인해 프로세스가 점진적으로 불안정해진다. Pioneer가 동일한 문제로 `pioneer-worker-budget`에서 100회 Dequeue 후 종료 정책을 도입한 것과 동일한 패턴을 Harvester에도 적용하여, 짧은 수명·외부 supervisor 재시작 모델로 운영 안정성을 확보한다.

본 change는 `pioneer-worker-budget`과 **대칭 정책**이다(DECISIONS §6 참조). 카운팅 규칙("성공 Dequeue만 카운트"), 정책 값(100, 빌드 상수, env 노출 금지), 체크 위치(성공 Dequeue 직후), graceful shutdown, supervisor 책임 등 모든 결정이 Pioneer와 동일하다. 운영자가 두 워커를 단일 멘탈 모델로 관리할 수 있도록 표현도 통일한다.

## What Changes

- Harvester 워커는 `URLScheduler.Dequeue`로부터 URL을 **성공적으로 수신한 횟수**를 기준으로 정확히 100회를 수행한 뒤 정상 종료(exit 0)한다.
- 카운터는 **성공 Dequeue 직후** 증가시키며, 빈 Dequeue(큐 비어 있음)와 오류 Dequeue(스케줄러 에러 반환)는 카운트하지 않는다.
- 100회째 Dequeue로 받은 URL의 harvest 작업은 중단하지 않고 끝까지 완료한다. 특히 `harvester_frontier` 갱신 및 `harvester_frontier_pins` INSERT가 트랜잭션으로 커밋된 뒤에만 종료한다(graceful shutdown).
- 워커 재시작은 외부 supervisor(예: systemd, k8s, docker restart policy)의 책임으로 명시한다. 워커 프로세스 자체는 재시작 로직을 갖지 않는다.
- budget 값(100)은 빌드 상수로 고정되며, 환경변수·설정으로 노출하지 않는다(Pioneer와 동일).

## Capabilities

### New Capabilities
- `harvester`: Harvester 워커 프로세스의 수명 정책. 본 change에서는 워커 budget(Dequeue 100회 후 종료) requirement만 정의한다. URLScheduler 소비 동작·콘텐츠 추출·재시도 정책 등은 다른 change(`harvester-scheduler-consumer`, `harvester-pin-document` 등)에서 다룬다.

### Modified Capabilities
<!-- 없음. 본 change는 신규 harvester capability에만 requirement를 추가한다. -->

## Impact

- **코드**: Harvester 워커 진입점(예: `apps/api/cmd/harvester/main.go` 또는 `apps/api/internal/bot/harvester/runner.go`)에 Dequeue 카운터와 종료 분기 추가. 인메모리 변수로 관리(공유 상태 없음).
- **운영**: Harvester를 supervisor 하에 배포해야 한다. 재시작 정책이 없는 환경에서는 워커가 100회 후 멈추므로 배포 가이드 갱신 필요.
- **관찰성**: 종료 직전 structured log 1줄(예: `msg="harvester worker: work budget exhausted" reason=budget_exhausted dequeues=100`)을 남겨 supervisor 재시작과 정상 종료를 구분 가능하게 한다. 로그 필드명은 Pioneer와 동일하게 맞춘다.
- **범위 외**: `pioneer-worker-budget`의 정책 변경, `URLScheduler` 자체 동작, Harvester 콘텐츠 추출/Pin 생성 로직 변경.
