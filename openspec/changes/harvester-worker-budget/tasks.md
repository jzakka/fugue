## 1. Harvester 워커 진입점에 budget 카운터 추가

- [ ] 1.1 Harvester 워커 진입점(예: `apps/api/cmd/harvester/main.go` 또는 동등 위치) 식별 및 현재 Dequeue 루프 구조 확인
- [ ] 1.2 budget 상수 정의: `const harvesterDequeueBudget = 100` (빌드 시 상수, env/설정/CLI 플래그로 노출 금지 — Pioneer와 동일)
- [ ] 1.3 Dequeue 루프를 카운터 기반으로 변경: **성공 Dequeue 직후** 카운터 증가 (URL을 실제로 반환한 호출만 카운트)
- [ ] 1.4 카운터가 100에 도달하면 진행 중인 harvest 작업(프론티어 갱신 및 `harvester_frontier_pins` INSERT 트랜잭션 커밋 포함)을 완료한 뒤 루프 탈출
- [ ] 1.5 빈 Dequeue 결과(URL 미반환) 시 카운터를 증가시키지 않는다 — 스케줄러 내부 blocking으로 인해 실질적으로는 드물지만 방어적으로 처리
- [ ] 1.6 Dequeue 자체 오류 반환 시 카운터를 증가시키지 않고, Pioneer와 동일한 structured 로그 필드(`component=harvester_worker reason=dequeue_error err=...`)로 로깅 후 재시도

## 2. 종료 처리

- [ ] 2.1 budget 소진 시 종료 직전 structured log 1줄 출력 — Pioneer와 동일 포맷: `msg="harvester worker: work budget exhausted" component=harvester_worker reason=budget_exhausted dequeues=100`
- [ ] 2.2 `os.Exit(0)` 또는 main 정상 반환으로 exit code 0 보장
- [ ] 2.3 워커가 자기 자신을 fork/exec하지 않는지 확인 (재시작 로직 부재 검증)
- [ ] 2.4 로그 필드명·레벨을 Pioneer worker-budget 로그와 통일되게 맞춤 (`component`, `reason`, `dequeues` 키 일치)

## 3. 운영 가이드 정합성

- [ ] 3.1 `docker-compose.yml`의 Harvester 서비스에 `restart: always` (또는 동등 정책)가 설정되어 있는지 확인하고 없으면 추가
- [ ] 3.2 `docs/architecture.md` 또는 README의 Harvester 운영 절에 "워커는 100회 처리 후 종료되며 supervisor가 재시작한다"를 명시
- [ ] 3.3 로컬 개발 가이드에 supervisor 없이 실행하는 방법(예: `while true; do harvester; done`) 한 줄 추가

## 4. 검증

- [ ] 4.1 단위 테스트: 100회 성공 Dequeue 후 루프가 종료되는지 (모킹된 URLScheduler 사용)
- [ ] 4.2 단위 테스트: 빈 Dequeue(URL 미반환)가 카운터를 증가시키지 않는지
- [ ] 4.3 단위 테스트: Dequeue 오류 반환이 카운터를 증가시키지 않는지 (오류 주입 mock → 재시도되며 카운터 불변 검증)
- [ ] 4.4 단위 테스트: 100회째 작업이 `harvester_frontier` 갱신 및 `harvester_frontier_pins` INSERT 트랜잭션 커밋까지 완료된 뒤에 종료가 발생하는지 (작업 진행 중 종료 금지)
- [ ] 4.5 단위 테스트: 종료 직전 로그가 정확히 1회, Pioneer와 동일한 필드(`reason=budget_exhausted dequeues=100`)로 출력되는지
- [ ] 4.6 로컬 통합 실행: Harvester 워커를 supervisor(또는 셸 루프) 하에 띄워 100회마다 재시작이 발생하는지 로그로 확인
- [ ] 4.7 `openspec validate harvester-worker-budget --strict` 통과 확인

## 5. 후속 change를 위한 메모 (본 change 범위 외)

- [ ] 5.1 budget 값 조정이 필요해질 경우 별도 change에서 다룬다는 점을 PR 설명에 명시
- [ ] 5.2 SIGTERM grace period, 빈 Dequeue 시 sleep 산식 등 운영 튜닝은 `scheduler-retry-backoff` 또는 별도 change에서 다룬다는 점을 PR 설명에 명시
