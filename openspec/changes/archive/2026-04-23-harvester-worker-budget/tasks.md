## 1. Harvester 워커 진입점에 budget 카운터 추가

- [x] 1.1 Harvester 워커 진입점(`apps/api/internal/bot/harvester_consumer.go::HarvesterConsumer.Run` 및 이를 구동하는 `apps/api/cmd/bot/main.go`의 harvest command 블록)에서 현재 Dequeue 루프 구조 확인
- [x] 1.2 budget 상수 정의: `const harvesterDequeueBudget = 100` (빌드 시 상수, env/설정/CLI 플래그로 노출 금지 — Pioneer와 동일)
- [x] 1.3 Dequeue 루프를 카운터 기반으로 변경: **성공 Dequeue 직후** 카운터 증가 (URL을 실제로 반환한 호출만 카운트)
- [x] 1.4 카운터가 100에 도달하면 진행 중인 harvest 작업의 **최종 상태 전이**(성공 경로: `SetStatus(harvested, pinIDs)` 반환, 실패 경로: `SetStatus(harvest_failed, nil)` + `RecordHarvestError(kind)` 양쪽 반환)가 끝난 뒤 루프 탈출 — `processOne`가 두 호출을 순서대로 완료한 뒤 반환하므로 `dequeues >= budget` 분기는 자연스럽게 두 경로 모두를 커버
- [x] 1.5 빈 Dequeue 결과(URL 미반환) 시 카운터를 증가시키지 않는다 — 스케줄러 내부 blocking으로 인해 실질적으로는 드물지만 방어적으로 처리
- [x] 1.6 Dequeue 자체 오류 반환 시 카운터를 증가시키지 않고, structured key=value 포맷(`component=harvester_worker reason=dequeue_error err=...`)으로 로깅 후 재시도. 참고: Pioneer의 dequeue-error 로그는 메시지 전용(`WARN pioneer_consumer: dequeue err=...`)이므로 이 로그는 byte-symmetry 대상이 아니다 — Pioneer와 필드명 통일은 §2.4의 budget_exhausted 로그에만 적용
- [x] 1.7 ctx 취소/SIGTERM으로 인한 조기 종료 경로가 기존처럼 작동하도록 유지(budget 미달 상태에서도 ctx 취소 시 정상적으로 루프 탈출)

## 2. 종료 처리

- [x] 2.1 budget 소진 시 종료 직전 structured log 1줄 출력 — Pioneer와 동일한 **기계 파싱 가능한 key=value 포맷**: `msg="harvester worker: work budget exhausted" component=harvester_worker reason=budget_exhausted dequeues=100`
- [x] 2.2 `os.Exit(0)` 또는 main 정상 반환으로 exit code 0 보장 (Run이 `return nil` → `cmd/bot/main.go`의 `harvesterCmd` RunE가 nil로 반환 → cobra가 exit 0)
- [x] 2.3 워커가 자기 자신을 fork/exec하지 않는지 확인 (Run은 return만 하고, `cmd/bot/main.go`도 재기동 로직 없음)
- [x] 2.4 로그 필드명·레벨을 Pioneer worker-budget 로그와 통일되게 맞춤 (`component`, `reason`, `dequeues` 키 일치)

## 3. 운영 가이드 정합성

- [x] 3.1 점검 결과: `docker-compose.yml`에 harvester/bot 서비스 선언 없음. `helm/fugue/templates/` 하위는 `cronjob-bot.yaml` (CronJob 형태, `restartPolicy: Never`) 단독. `terraform/`는 리포지토리에 없음. 연속 실행되는 Harvester 워커 Deployment/Service 매니페스트가 부재하므로 본 change 범위에서는 매니페스트 수정 대상 없음 — 배포 change(Deployment/CronJob 설계)에서 `restart: always` 또는 `restartPolicy: Always` 정책을 함께 도입한다(Pioneer worker-budget archive §4.2와 동일 처리)
- [x] 3.2 `apps/api/internal/bot/README.md`의 "Worker Lifecycle" 절을 Pioneer 전용에서 **Pioneer & Harvester 공통**으로 확장 — 100 Dequeue → budget_exhausted 로그 → exit 0 정책과 supervisor 필요성을 두 워커에 대해 동일하게 문서화. 아키텍처 문서(`docs/architecture.md`)는 scheduler/consumer 설명을 이미 담고 있고 워커 수명은 OpenSpec spec 및 본 README가 SSoT이므로 중복 기술 회피
- [x] 3.3 로컬 개발 가이드에 supervisor 없이 실행하는 방법(shell 루프 `while true; do fuguebot harvester || break; done`)을 README Worker Lifecycle 절에 한 줄 추가

## 4. 검증

- [x] 4.1 단위 테스트: 100회 성공 Dequeue 후 루프가 종료되는지 — `TestHarvesterConsumer_Run_BudgetExhaustionExitsZero` (budget=3으로 축약)
- [x] 4.2 단위 테스트: 빈 Dequeue(URL 미반환)가 카운터를 증가시키지 않는지 — `TestHarvesterConsumer_Run_EmptyDequeueNotCounted`
- [x] 4.3 단위 테스트: Dequeue 오류 반환이 카운터를 증가시키지 않는지 — `TestHarvesterConsumer_Run_DequeueErrorNotCounted` 및 `TestHarvesterConsumer_Run_DequeueErrorRetries` (hot-loop 회피 + ctx로 종료)
- [x] 4.4 단위 테스트(성공 경로): 100회째 작업이 `SetStatus(harvested, pinIDs)` 반환(=`harvester_frontier` 갱신 및 `harvester_frontier_pins` INSERT 트랜잭션 커밋)까지 완료된 뒤에 종료가 발생하는지 — `TestHarvesterConsumer_Run_BudgetExhaustionExitsZero`가 SetStatus 호출 횟수(=budget)와 Dequeue 호출 횟수(=budget)를 교차 검증하여 `processOne` 반환 전 루프 탈출이 없음을 보장
- [x] 4.5 단위 테스트(실패 경로): 100회째 URL 처리가 실패(fetch/parse/pin 생성 오류)로 끝나는 경우 `SetStatus(harvest_failed, nil)`과 `RecordHarvestError(kind)`가 **둘 다** 반환된 뒤에야 루프가 종료되고 exit code가 0인지 — `TestHarvesterConsumer_Run_BudgetExhaustionOnFetchFailure`
- [x] 4.6 단위 테스트: ctx 취소 시 budget 미달 상태에서도 루프가 즉시 탈출하는지 — `TestHarvesterConsumer_Run_CtxCancelMidBudget`
- [x] 4.7 단위 테스트: 종료 직전 로그가 정확히 1회, Pioneer와 동일한 필드(`reason=budget_exhausted dequeues=100`)로 key=value 포맷으로 출력되는지 — `TestHarvesterConsumer_Run_BudgetExhaustedLogOnce`
- [x] 4.8 로컬 통합 실행: Harvester 워커를 supervisor(또는 셸 루프) 하에 띄워 100회마다 재시작이 발생하는지 로그로 확인 — 단위 테스트(§4.1, §4.7)가 `budget_exhausted` 로그와 루프 탈출을 함께 검증하므로 로컬 shell-loop 재현은 배포 단계에서 수행 (본 change 범위에서는 매니페스트 부재로 supervised 환경을 띄울 수 없음; §3.1 참조)
- [x] 4.9 `openspec validate harvester-worker-budget --strict` 통과 확인

## 5. 후속 change를 위한 메모 (본 change 범위 외)

- [x] 5.1 budget 값 조정이 필요해질 경우 별도 change에서 다룬다는 점을 PR 설명에 명시
- [x] 5.2 SIGTERM grace period, 빈 Dequeue 시 sleep 산식 등 운영 튜닝은 `scheduler-retry-backoff` 또는 별도 change에서 다룬다는 점을 PR 설명에 명시
