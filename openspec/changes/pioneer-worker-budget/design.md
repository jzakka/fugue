## Context

Pioneer는 URLScheduler로부터 URL을 Dequeue하여 크롤링하는 consumer 워커이다(관련 change: `pioneer-scheduler-consumer`). 운영상 **여러 Pioneer 프로세스가 동시에 실행**되며, 각 프로세스는 자신의 루프에서 반복적으로 Dequeue → fetch → extract → enqueue(링크) 사이클을 수행한다.

장기 실행 Go 프로세스는 다음 위험에 노출된다:
- 힙 단편화 및 GC 압력 누적
- HTML 파서·HTTP 클라이언트·TLS 세션 등 라이브러리 내부 상태 누수
- 전역 캐시나 visited 맵 등 인메모리 구조가 세션 범위를 넘어 커지는 드리프트
- 간헐적 goroutine 누수로 인한 리소스 leak

이를 근본적으로 제거하기보다, **crash-only / periodic restart** 패턴으로 방어한다. 워커는 제한된 "작업 예산(work budget)"만 소비하고 깔끔하게 종료하며, supervisor가 새 프로세스를 띄운다. 이는 Erlang의 "let it crash"와 Kubernetes liveness/restart policy에서 차용한 보편적 운영 패턴이다.

관련 제약:
- Pioneer 상태는 이미 인메모리(세션 스코프)로 관리됨(기존 spec: "DB 기존 노드와 무관하게 BFS 큐 관리"). 따라서 재시작 시 상태 손실이 정상 동작이다.
- 복수 Pioneer 프로세스 환경이므로 budget 카운터는 **프로세스 로컬**이어야 한다.

## Goals / Non-Goals

**Goals:**
- Pioneer 워커가 **Dequeue 루프 100회 반복 후 정상 종료**(exit 0)
- 예산 소진 시점에 **진행 중인 URL 처리는 완료**한 뒤 종료(graceful, mid-flight abort 금지)
- 종료 사유를 로그로 남겨 운영자가 재시작 사이클을 관측 가능하게 함
- supervisor 기반 재시작 전제의 운영 모델을 명시적으로 문서화

**Non-Goals:**
- supervisor/오케스트레이터(systemd, k8s, docker restart) 구현
- Harvester 워커의 수명 관리(별도 변경으로 다룸)
- URLScheduler 자체의 수명/지속성 관리
- 동적 budget 조정, 시간 기반 종료, 메모리 기반 종료 등 고도화된 정책
- Warm restart, 상태 핸드오프, 프로세스 간 budget 공유

## Decisions

### 결정 1: work budget은 "Dequeue 루프 반복 횟수"로 측정

- **선택**: Dequeue 호출 성공 여부와 무관하게 **루프 이터레이션 1회 = 1 카운트**. 상한 100회 도달 시 종료.
- **대안 A**: 처리 성공한 URL 수만 카운트 — Dequeue 빈 응답(idle)도 수명에 기여하지 않으므로 장기 idle 상태 워커가 영원히 살아남는 문제 발생.
- **대안 B**: 경과 시간(예: 30분) — 처리량에 무관하게 종료되어 바쁜 워커가 과도하게 재시작됨. 처리량 기반(루프 카운트)이 부하에 비례.
- **대안 C**: 메모리 임계치 — 구현 복잡도 높고 플랫폼 의존적.
- 100이라는 숫자는 사용자 결정값이며, 향후 튜닝은 별도 변경에서 다룬다.

### 결정 2: graceful shutdown — 진행 중 URL은 완료 후 종료

- 카운터 체크는 **Dequeue 이전(루프 상단)** 에서 수행한다. 이미 Dequeue한 URL은 fetch/extract/enqueue 사이클을 끝까지 수행한 뒤 루프를 빠져나온다.
- 이유: mid-flight 중단 시 scheduler에 URL이 dequeued 상태로 남거나, 부분적으로 추출한 링크가 유실될 수 있다. "단위 작업"을 원자적으로 끝내는 것이 scheduler와의 계약을 단순하게 유지한다.

### 결정 3: 종료는 프로세스 exit 0

- 정상 종료(exit code 0)로 supervisor가 "성공적 완료 후 재시작" 또는 "항상 재시작" 정책 어느 쪽에서도 동일하게 동작하게 한다.
- panic이나 non-zero exit은 운영자에게 이상 신호로 유지한다(예산 소진은 이상 신호가 아니다).

### 결정 4: budget은 프로세스 로컬, 인메모리 카운터

- 복수 프로세스 환경에서 각 워커는 독립적으로 자신의 100회를 센다.
- DB/Redis 기반 공유 카운터는 도입하지 않는다(불필요한 복잡성, scheduler 책임 아님).

### 결정 5: supervisor는 스펙 범위 밖

- 재시작 책임은 런타임 환경(systemd Restart=always, k8s Deployment restartPolicy, docker --restart 등)에 위임.
- 스펙은 "워커는 종료한다"까지 정의하고, "누가 다시 띄우는가"는 배포 설정의 문제로 분리.

## Risks / Trade-offs

- **[Risk] 100회가 너무 작으면 재시작 오버헤드(프로세스 기동, scheduler 재연결, TLS handshake)가 처리량을 잠식** → 초기값 100으로 시작하고 로그/메트릭으로 재시작 빈도 관측 후 튜닝. 튜닝은 별도 change.
- **[Risk] 100회가 너무 커서 메모리 누수가 축적될 수 있음** → budget 도입 자체가 상한이므로 무한 성장 방지. 관측을 통해 조정.
- **[Risk] supervisor가 없는 환경에서 실행 시 워커가 단발성으로 끝남** → 운영 문서에 supervisor 필수 의존을 명시. 스펙도 "supervisor 책임"을 명문화.
- **[Risk] graceful 종료 중 진행 중 URL이 오래 걸리면 종료 지연** → 기존 Pioneer의 fetch 타임아웃이 이미 존재하므로 최악의 경우도 유한 시간 내 종료.
- **[Trade-off] 메모리 누수를 진단·수정하는 대신 주기적 재시작으로 회피** → 근본 원인 해결은 아니지만, 실제 운영에서는 저비용·고신뢰 패턴. 누수가 식별되면 별도 fix.
