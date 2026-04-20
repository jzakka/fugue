## 1. Pioneer 워커 진입점에 budget 카운터 추가

- [x] 1.1 Pioneer 워커 진입점(`apps/api/internal/bot/pioneer_consumer.go::PioneerConsumer.Run`, `apps/api/cmd/bot/main.go::runPioneerConsumer`) 식별 및 현재 Dequeue 루프 구조 확인
- [x] 1.2 budget 상수 정의: `const WorkerBudget = 100` (빌드 시 상수, env·config 노출 금지)
- [x] 1.3 Dequeue 루프를 카운터 기반으로 변경: **`URLScheduler.Dequeue`가 URL을 성공적으로 반환한 직후**에만 카운터 증가
- [x] 1.4 Dequeue가 오류를 반환한 경우 카운터를 증가시키지 않고 로깅 후 재시도 (idle 상태는 Dequeue 내부 blocking이 처리하므로 consumer는 별도 처리하지 않음)
- [x] 1.5 카운터가 `WorkerBudget`에 도달하면 **현재 URL의 fetch → 링크 추출 → Enqueue → SetStatus 사이클 전체를 완료한 뒤** 루프 탈출

## 2. Graceful Shutdown 경로

- [x] 2.1 루프 탈출 후 exit code 0으로 프로세스 종료 (`runPioneerConsumer`가 `Run`의 nil 반환을 그대로 RunE에 전파 → cobra가 exit 0)
- [x] 2.2 종료 직전 key=value 포맷 로그 1회 출력: Harvester와 대칭된 형식 — 메시지 `"pioneer worker: work budget exhausted"`, 필드 `component=pioneer_worker reason=budget_exhausted dequeues=100`
- [x] 2.3 종료 시점에 열려 있는 리소스(HTTP client, DB conn 등)가 기존 defer/Close 경로로 정리되는지 점검 — `runPioneerConsumer` 호출 사이트의 `defer infra.Close()` 가 그대로 정리 보장
- [x] 2.4 워커가 자기 자신을 fork/exec하지 않는지 확인 — `Run` 루프에 fork/exec 호출 없음

## 3. 테스트

- [x] 3.1 단위 테스트: 가짜 `URLScheduler`를 주입하여 **성공 Dequeue N회 수령** 후 루프가 종료됨을 검증 (`TestPioneerConsumer_Run_BudgetExhaustionExitsZero`, budget=3로 동등성 확인)
- [x] 3.2 단위 테스트: **빈 Dequeue는 카운트되지 않음** (`TestPioneerConsumer_Run_EmptyDequeueNotCounted`)
- [x] 3.3 단위 테스트: Dequeue가 오류를 반환하는 경로에서 카운터가 증가하지 않음을 검증 (`TestPioneerConsumer_Run_DequeueErrorNotCounted`)
- [x] 3.4 단위 테스트: **N-1회까지는 종료하지 않음** — 2/3까지 성공한 시점에 ctx 만료로만 종료됨을 검증 (`TestPioneerConsumer_Run_DoesNotExitBeforeBudget`)
- [x] 3.5 단위 테스트: **N회째 Dequeue로 받은 URL 처리 완료 후 exit 0** — 각 URL의 SetStatus가 호출된 뒤 종료 경로 실행 검증 (`TestPioneerConsumer_Run_BudgetExhaustionExitsZero`)
- [x] 3.6 단위 테스트: budget 완료 후 추가 Dequeue 호출이 발생하지 않음을 mock으로 검증 (`TestPioneerConsumer_Run_BudgetExhaustionExitsZero`의 dequeueCalls 검사)
- [x] 3.7 단위 테스트: 복수 워커 인스턴스가 각자 독립 카운터를 갖는지 검증 (`TestPioneerConsumer_Run_IndependentBudgetsPerInstance`)
- [x] 3.8 (추가) 기본 budget 상수가 100임을 회귀 방지 (`TestPioneerConsumer_Run_DefaultBudgetIsWorkerBudget`)
- [x] 3.9 (추가) budget 종료 시점 URL의 fetch 실패도 exit 0를 보장 (`TestPioneerConsumer_Run_BudgetExhaustionOnFetchFailure`) — spec scenario "100회째 처리 실패도 정상 종료"

## 4. 운영/문서화

- [x] 4.1 `apps/api/internal/bot/README.md`에 "Worker Lifecycle (PioneerConsumer)" 섹션 추가 — 100회 후 종료/supervisor 필요 명시
- [x] 4.2 실제 배포 경로(docker-compose / k8s manifest / systemd unit)에 재시작 정책(`restart: always` / `restartPolicy: Always` / `Restart=always`) 이 설정되어 있는지 확인 — 현재 `docker-compose.yml`에는 Pioneer 서비스가 정의되어 있지 않고, `helm/` 및 `terraform/` 경로에도 Pioneer 워커 리소스가 선언되지 않아 조치 대상 manifest 부재. 실제 배포 매체가 추가되는 시점에 supervisor 정책 설정은 해당 배포 change의 책임 범위이다 (README Worker Lifecycle 섹션에 supervisor 필수 의존 명시)
- [x] 4.3 README의 Worker Lifecycle 섹션에 shell loop / systemd / Docker compose 예시 추가
- [x] 4.4 budget 소진 로그 포맷이 Harvester와 동일한 필드 집합(`component`, `reason`, `dequeues`)을 갖도록 맞춤 — 코드/스펙 모두 적용

## 5. 검증

런타임/배포 검증(5.1–5.3)은 본 change의 스코프 밖이며, 단위 테스트로 동등성을 확보한다. 실제 Pioneer 서비스 매니페스트가 배포 파이프라인에 추가되는 시점에 별도 배포 change에서 담당한다.

- [x] 5.1 단위 테스트로 성공 Dequeue 100회 후 정상 종료 및 exit 0 동등성 확인 (`TestPioneerConsumer_Run_BudgetExhaustionExitsZero` 등) — 실제 런타임 검증은 배포 change로 이관
- [x] 5.2 단위 테스트로 `reason=budget_exhausted` 로그가 Run 종료 경로에서 정확히 1회 출력됨을 검증 (`TestPioneerConsumer_Run_BudgetExhaustedLogOnce` 에서 `log.Writer()`를 bytes.Buffer로 교체하여 출력 횟수·필드 집합 assert) — 실제 런타임 출력 확인은 배포 change로 이관
- [x] 5.3 supervisor 환경의 자동 재시작 검증은 본 change 범위 밖 — README에 supervisor 필수 의존을 명시하는 것으로 대체하며, 실제 배포 매니페스트 작성 시점의 배포 change에서 검증
- [x] 5.4 `openspec validate pioneer-worker-budget --strict` 통과 확인
