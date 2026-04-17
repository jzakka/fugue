## 1. Harvester 워커 진입점에 budget 카운터 추가

- [ ] 1.1 Harvester 워커 진입점(예: `apps/api/cmd/harvester/main.go` 또는 동등 위치) 식별 및 현재 Dequeue 루프 구조 확인
- [ ] 1.2 budget 상수 정의: `const harvesterDequeueBudget = 100` (빌드 시 상수, env 노출 금지)
- [ ] 1.3 Dequeue 루프를 카운터 기반으로 변경: 성공적으로 URL을 받은 호출에서만 카운터 증가
- [ ] 1.4 카운터가 100에 도달하면 진행 중인 harvest 작업 완료 후 루프 탈출
- [ ] 1.5 빈 Dequeue 결과 시 카운터를 증가시키지 않고 짧은 sleep 후 재시도
- [ ] 1.6 Dequeue 자체 오류 시 카운터를 증가시키지 않고 적절히 로깅 후 재시도

## 2. 종료 처리

- [ ] 2.1 budget 소진 시 종료 직전 명시적 로그 추가 (예: `harvester worker reached dequeue budget, exiting` + `reason=budget_exhausted`)
- [ ] 2.2 `os.Exit(0)` 또는 main 정상 반환으로 exit code 0 보장
- [ ] 2.3 워커가 자기 자신을 fork/exec하지 않는지 확인 (재시작 로직 부재 검증)

## 3. 운영 가이드 정합성

- [ ] 3.1 `docker-compose.yml`의 Harvester 서비스에 `restart: always` (또는 동등 정책)가 설정되어 있는지 확인하고 없으면 추가
- [ ] 3.2 `docs/architecture.md` 또는 README의 Harvester 운영 절에 "워커는 100회 처리 후 종료되며 supervisor가 재시작한다"를 명시
- [ ] 3.3 로컬 개발 가이드에 supervisor 없이 실행하는 방법(예: `while true; do harvester; done`) 한 줄 추가

## 4. 검증

- [ ] 4.1 단위 테스트: 100회 Dequeue 후 루프가 종료되는지 (모킹된 URLScheduler 사용)
- [ ] 4.2 단위 테스트: 빈 Dequeue가 카운터를 증가시키지 않는지
- [ ] 4.3 단위 테스트: Dequeue 오류가 카운터를 증가시키지 않는지
- [ ] 4.4 단위 테스트: 100회째 작업이 완료된 뒤에 종료가 발생하는지 (작업 진행 중 종료가 일어나지 않는지)
- [ ] 4.5 로컬 통합 실행: Harvester 워커를 supervisor(또는 셸 루프) 하에 띄워 100회마다 재시작이 발생하는지 로그로 확인
- [ ] 4.6 `openspec validate harvester-worker-budget --strict` 통과 확인

## 5. 후속 change를 위한 메모 (본 change 범위 외)

- [ ] 5.1 budget 값 조정이 필요해질 경우 별도 change에서 다룬다는 점을 PR 설명에 명시
- [ ] 5.2 SIGTERM grace period, 빈 Dequeue 시 sleep 산식 등 운영 튜닝은 `scheduler-retry-backoff` 또는 별도 change에서 다룬다는 점을 PR 설명에 명시
