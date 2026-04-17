## Why

Pioneer 워커는 URLScheduler consumer로서 Dequeue 루프를 장시간 돌리는 장기 실행 프로세스이다. 장시간 실행은 Go 힙 단편화, 파서/HTTP 클라이언트 내부 상태 누적, 의도치 않은 상태 드리프트(전역 캐시 증가 등)를 유발할 수 있다. 이를 방어하기 위해 **워커 수명(work budget)** 을 명시적으로 제한하고, 소진 시 정상 종료하여 supervisor가 새 프로세스로 재시작하는 "crash-only" 스타일의 운영 규약을 확립한다.

## What Changes

- Pioneer 워커에 **Dequeue 루프 100회 상한**(work budget) 도입
- 상한 도달 시 **진행 중 URL 처리 완료 후 정상 종료**(exit code 0)
- 워커는 **상태 없이(stateless)** 종료하여 supervisor(스펙 범위 밖)가 재시작 담당
- 종료 직전 budget-소진 로그를 남겨 재시작 사이클을 관측 가능하게 함

범위 외: Harvester 워커 budget, URLScheduler 자체 수명 관리, supervisor/오케스트레이터 구현.

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities
- `bot`: Pioneer 워커에 Dequeue 100회 work budget과 graceful shutdown 요건 추가

## Impact

- **코드**: `apps/api/internal/bot/` — Pioneer 워커 루프에 Dequeue 카운터 및 종료 경로 추가
- **코드**: `apps/api/cmd/bot/main.go` (또는 Pioneer 엔트리포인트) — budget 소진 시 exit 0로 반환
- **운영**: 다수 Pioneer 프로세스를 재시작 가능한 supervisor(systemd, k8s Deployment, Docker restart policy 등)에서 실행하는 것을 전제
- **인메모리 상태**: budget 카운터는 프로세스 로컬이며, 세션 간 공유되지 않음. Dequeue/visited 맵 등 기존 인메모리 구조는 변경 없음
- **관측성**: budget 소진 종료 이벤트 로그 추가
