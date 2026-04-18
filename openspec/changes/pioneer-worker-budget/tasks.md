## 1. Pioneer 워커 진입점에 budget 카운터 추가

- [ ] 1.1 Pioneer 워커 진입점(예: `apps/api/cmd/bot/main.go` 또는 `apps/api/internal/bot/pioneer/runner.go` 등) 식별 및 현재 Dequeue 루프 구조 확인
- [ ] 1.2 budget 상수 정의: `const WorkerBudget = 100` (빌드 시 상수, env·config 노출 금지)
- [ ] 1.3 Dequeue 루프를 카운터 기반으로 변경: **`URLScheduler.Dequeue`가 URL을 성공적으로 반환한 직후**에만 카운터 증가
- [ ] 1.4 Dequeue가 오류를 반환한 경우 카운터를 증가시키지 않고 로깅 후 재시도 (idle 상태는 Dequeue 내부 blocking이 처리하므로 consumer는 별도 처리하지 않음)
- [ ] 1.5 카운터가 `WorkerBudget`에 도달하면 **현재 URL의 fetch → 링크 추출 → Enqueue → SetStatus 사이클 전체를 완료한 뒤** 루프 탈출

## 2. Graceful Shutdown 경로

- [ ] 2.1 루프 탈출 후 exit code 0으로 프로세스 종료 (cmd 엔트리에서 정상 반환 경로 확인)
- [ ] 2.2 종료 직전 구조화 로그 1회 출력: Harvester와 통일된 형식 — 메시지 `"pioneer worker reached dequeue budget, exiting"`, 필드 `reason=budget_exhausted`
- [ ] 2.3 종료 시점에 열려 있는 리소스(HTTP client, DB conn 등)가 기존 defer/Close 경로로 정리되는지 점검
- [ ] 2.4 워커가 자기 자신을 fork/exec하지 않는지 확인 (재시작 로직 부재 검증)

## 3. 테스트

- [ ] 3.1 단위 테스트: 가짜 `URLScheduler`를 주입하여 **성공 Dequeue 100회 수령** 후 루프가 종료됨을 검증 (각 URL의 fetch/Enqueue/SetStatus가 호출된 뒤 종료)
- [ ] 3.2 단위 테스트: **빈 Dequeue는 카운트되지 않음** — Dequeue가 내부 blocking이므로 실제로 소비자에 빈 결과가 노출되지는 않지만, 해당 경로가 테스트용으로 시뮬레이션될 때 카운터가 증가하지 않음을 확인
- [ ] 3.3 단위 테스트: Dequeue가 오류를 반환하는 경로에서 카운터가 증가하지 않음을 검증
- [ ] 3.4 단위 테스트: **99회까지는 종료하지 않음** — 99회 성공 Dequeue 후에도 다음 Dequeue를 시도함을 검증
- [ ] 3.5 단위 테스트: **100회째 Dequeue로 받은 URL 처리 완료 후 exit 0** — 100회째 URL의 fetch/링크 추출/Enqueue/SetStatus가 모두 호출된 뒤 종료 경로가 실행됨을 검증
- [ ] 3.6 단위 테스트: 100회 완료 후 추가 Dequeue 호출이 발생하지 않음을 mock으로 검증
- [ ] 3.7 단위 테스트: 복수 워커 인스턴스가 각자 독립 카운터를 갖는지 검증 (프로세스 로컬 상태)

## 4. 운영/문서화

- [ ] 4.1 Pioneer 실행 문서(README 또는 `docs/architecture.md` 관련 섹션)에 "워커는 성공 Dequeue 100회 후 종료하므로 supervisor 필요"를 명시 (Harvester 운영 가이드와 동일 문구 사용)
- [ ] 4.2 `docker-compose.yml`의 Pioneer 서비스에 `restart: always` (또는 동등 정책)가 설정되어 있는지 확인하고 없으면 추가
- [ ] 4.3 로컬 실행 시 간단한 쉘 루프(`while true; do fuguebot pioneer ...; done`) 또는 systemd/docker restart 예시를 운영 노트에 추가
- [ ] 4.4 budget 소진 로그가 Harvester 로그 포맷과 필드명 일관성을 유지하도록 맞춤

## 5. 검증

- [ ] 5.1 로컬에서 Pioneer 실행하여 성공 Dequeue 100회 후 정상 종료 및 exit 0 확인
- [ ] 5.2 로그에 `reason=budget_exhausted` 메시지가 정확히 1회 출력되는지 확인
- [ ] 5.3 supervisor 환경(로컬 쉘 루프 또는 docker restart)에서 자동 재시작이 동작하는지 확인
- [ ] 5.4 `openspec validate pioneer-worker-budget --strict` 통과 확인
