## 1. Pioneer 워커 루프 수정

- [ ] 1.1 Pioneer Dequeue 루프의 엔트리포인트 위치 확인 (`apps/api/internal/bot/` 내 pioneer consumer/worker 구현체)
- [ ] 1.2 루프 진입 전 `workBudget := 100` 상수/설정값 도입 (내부 상수로 시작, 추후 설정화 여지 남김)
- [ ] 1.3 루프 상단에 `iteration` 카운터 도입 및 `iteration >= workBudget`일 때 break하는 조건 추가
- [ ] 1.4 카운터 증가는 **Dequeue 직전**에서 1회 수행(idle 응답도 포함되도록)
- [ ] 1.5 이미 Dequeue된 URL의 fetch/extract/enqueue 사이클이 완료된 후에만 break가 반영되도록 제어 흐름 검증

## 2. Graceful Shutdown 경로

- [ ] 2.1 루프 탈출 후 exit code 0으로 프로세스 종료(cmd/bot 엔트리에서 정상 반환 경로 확인)
- [ ] 2.2 종료 직전 "pioneer worker: work budget exhausted (iterations=100)" 취지의 로그 출력
- [ ] 2.3 종료 시점에 열려 있는 리소스(HTTP client, DB conn 등)가 기존 defer/Close 경로로 정리되는지 점검

## 3. 테스트

- [ ] 3.1 단위 테스트: 가짜 scheduler를 주입하여 Dequeue가 100회 호출된 뒤 루프가 종료함을 검증
- [ ] 3.2 단위 테스트: Dequeue가 빈 응답만 반환하는 idle 시나리오에서도 100회 후 종료함을 검증
- [ ] 3.3 단위 테스트: 99회 시점에서 Dequeue한 URL의 처리가 진행 중일 때, 처리 완료 후 종료함을 검증(mid-flight 중단 없음)
- [ ] 3.4 단위 테스트: 100회 완료 후 추가 Dequeue 호출이 발생하지 않음을 mock으로 검증
- [ ] 3.5 단위 테스트: 복수 워커 인스턴스가 각자 독립 카운터를 갖는지 검증(프로세스 로컬 상태)

## 4. 운영/문서화

- [ ] 4.1 Pioneer 실행 문서(README 또는 AGENTS.md 관련 섹션)에 "워커는 100회 후 종료하므로 supervisor 필요"를 명시
- [ ] 4.2 로컬 실행 시 간단한 쉘 루프(`while true; do fuguebot pioneer ...; done`) 또는 systemd/docker restart 예시를 운영 노트에 추가
- [ ] 4.3 budget 소진 로그가 기존 로그 포맷(구조화 로그라면 필드명 일관성)과 어긋나지 않도록 맞춤

## 5. 검증

- [ ] 5.1 로컬에서 Pioneer 실행하여 100회 후 정상 종료 및 exit 0 확인
- [ ] 5.2 로그에 budget 소진 메시지가 정확히 1회 출력되는지 확인
- [ ] 5.3 supervisor 환경(로컬 쉘 루프 또는 docker restart)에서 자동 재시작이 동작하는지 확인
- [ ] 5.4 `openspec validate pioneer-worker-budget` 통과 확인
