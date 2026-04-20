## Context

Pioneer는 URLScheduler로부터 URL을 Dequeue하여 크롤링하는 consumer 워커이다(관련 change: `pioneer-scheduler-consumer`). 운영상 **여러 Pioneer 프로세스가 동시에 실행**되며, 각 프로세스는 자신의 루프에서 반복적으로 Dequeue → fetch → 링크 추출 → Enqueue → SetStatus 사이클을 수행한다.

장기 실행 Go 프로세스는 다음 위험에 노출된다:
- 힙 단편화 및 GC 압력 누적
- HTML 파서·HTTP 클라이언트·TLS 세션 등 라이브러리 내부 상태 누수
- 전역 캐시나 visited 맵 등 인메모리 구조가 세션 범위를 넘어 커지는 드리프트
- 간헐적 goroutine 누수로 인한 리소스 leak

이를 근본적으로 제거하기보다, **crash-only / periodic restart** 패턴으로 방어한다. 워커는 제한된 "작업 예산(work budget)"만 소비하고 깔끔하게 종료하며, supervisor가 새 프로세스를 띄운다. 이는 Erlang의 "let it crash"와 Kubernetes liveness/restart policy에서 차용한 보편적 운영 패턴이다.

본 change는 `harvester-worker-budget`과 **완전히 동일한 정책**을 Pioneer에 적용하여, 운영자가 두 워커의 수명 관리를 단일 멘탈 모델로 이해할 수 있게 한다.

관련 제약:
- Pioneer 상태는 이미 인메모리(세션 스코프)로 관리됨. 따라서 재시작 시 상태 손실이 정상 동작이다.
- 복수 Pioneer 프로세스 환경이므로 budget 카운터는 **프로세스 로컬**이어야 한다.
- `URLScheduler.Dequeue`는 내부 blocking 방식(빈 큐면 sleep 후 재시도)이므로, 소비자에게 "빈 결과"가 반환되지 않는다. 카운팅 시 이 계약을 전제로 한다.

## Goals / Non-Goals

**Goals:**
- Pioneer 워커가 **성공 Dequeue 100회 후 정상 종료**(exit 0)
- 예산 소진 시점에 **진행 중인 URL 처리(fetch/링크 추출/Enqueue/SetStatus)는 완료**한 뒤 종료(graceful, mid-flight abort 금지)
- 종료 사유를 로그로 남겨 운영자가 재시작 사이클을 관측 가능하게 함
- supervisor 기반 재시작 전제의 운영 모델을 명시적으로 문서화
- Harvester 워커와 **정책·카운팅 기준·증가 위치·종료 판정 위치·로그 포맷 동일**

**Non-Goals:**
- supervisor/오케스트레이터(systemd, k8s, docker restart) 구현
- Harvester 워커의 수명 관리(`harvester-worker-budget`에서 별도 정의 — 본 change 작성 시점에도 active 상태이며 아카이브 순서는 상호 독립이다. 두 change 사이의 스펙/로그 포맷 일치는 `harvester-worker-budget` 아카이브 시점에 교차 검증한다)
- URLScheduler 자체의 수명/지속성 관리
- 동적 budget 조정, 시간 기반 종료, 메모리 기반 종료 등 고도화된 정책
- Warm restart, 상태 핸드오프, 프로세스 간 budget 공유
- budget 값의 env/config 노출 (명시적으로 금지)

## Decisions

### Decision 1: work budget 카운팅은 "성공한 Dequeue만" 센다

**선택**: `URLScheduler.Dequeue`가 **URL을 실제로 반환한 호출**에 한해서만 카운터를 1 증가시킨다. 빈 결과나 오류를 반환한 Dequeue 호출은 카운트하지 않는다. 상한 100 도달 시 종료.

**대안 (폐기)**:
- ~~(A) 루프 이터레이션 1회 = 1카운트~~ — 폐기. `URLScheduler.Dequeue`가 내부 blocking이라 "빈 이터레이션"이 consumer에게 노출되지 않으므로 구분이 모호하고, `harvester-worker-budget`(현재 active)가 제안하는 "성공 Dequeue만 카운트" 기준과 어긋남.
- (B) 시간 기반 종료(예: 30분) — 처리량에 무관하게 종료되어 바쁜 워커가 과도하게 재시작됨. 처리량 기반(Dequeue 카운트)이 부하에 비례.
- (C) 메모리 임계치 — 구현 복잡도 높고 플랫폼 의존적.

**근거**: Harvester와 **동일한 카운팅 기준**을 사용하여 단일 멘탈 모델 유지. Dequeue 내부가 blocking이므로 "빈 결과는 카운트하지 않는다" 규칙은 실질적으로 "URL을 받으면 1카운트"로 환원된다. 다만 Dequeue가 에러를 반환하는 드문 경로가 생기더라도 해당 호출은 카운트에서 제외된다.

### Decision 2: 카운터 증가는 "Dequeue 성공 직후", 종료 판정은 "현재 URL 사이클 완료 후"

**선택**: 카운터 증가는 **Dequeue가 URL을 반환하여 성공한 직후**에 수행하며, 종료 임계 판정(break 결정)은 해당 URL의 fetch/링크 추출/Enqueue/SetStatus 사이클이 끝난 뒤 다음 루프 반복 시작 전에 "임계 도달이면 break" 한다. 증가 시점과 판정 시점이 분리된다는 점이 핵심이다.

**대안 (폐기)**:
- ~~(A) Dequeue 이전 루프 상단에서 종료 판정~~ — 폐기. 성공/빈/오류 구분 없이 세는 옛 방식. Harvester와 불일치.
- (B) 처리 완료 후에 종료 판정 — 채택안과 실질 동일 (100회째 처리 후 즉시 break)

**근거**: "성공 Dequeue만 카운트"와 "진행 중 작업 미중단" 두 요구를 동시에 만족. 100회째 URL을 받으면 카운터가 100이 되고, 해당 URL 처리가 끝난 뒤 루프 조건 검사에서 종료가 결정된다.

### Decision 3: graceful shutdown — 진행 중 URL은 완료 후 종료

**선택**: 100회째 Dequeue로 받은 URL의 fetch, 링크 추출, Enqueue(신규 링크 재투입), SetStatus(frontier 갱신)까지 모두 끝낸 뒤 루프를 빠져나와 프로세스를 종료한다.

**근거**: mid-flight 중단 시 scheduler에 URL이 claim된 채로 lease timeout(10분)까지 대기하게 되고, 부분적으로 추출한 링크가 유실될 수 있다. "단위 작업"을 원자적으로 끝내는 것이 scheduler와의 계약을 단순하게 유지한다. Harvester의 정책과 동일.

### Decision 4: 종료는 프로세스 exit 0

**선택**: 정상 종료(exit code 0)로 supervisor가 "정상 종료 시 즉시 재시작" 정책(systemd `Restart=always`, k8s `restartPolicy: Always` 등)에서 동일하게 동작하게 한다.

**근거**: budget 소진은 비정상 상태가 아니다. 종료 사유 구분이 필요할 때는 로그(`reason=budget_exhausted`)로 충분하다. Harvester와 동일.

### Decision 5: budget 값은 빌드 상수, env/config 노출 금지

**선택**: `const WorkerBudget = 100`과 같이 **빌드 시 상수**로 고정한다. 환경변수·설정 파일·CLI 플래그로 런타임 변경을 허용하지 않는다.

**대안 (폐기)**:
- ~~(A) `PIONEER_WORKER_BUDGET` env로 노출~~ — 폐기. `harvester-worker-budget`(현재 active)도 env 미노출을 제안 중이며, Pioneer만 노출하면 두 워커 정책이 분기됨.
- (B) 본 change 채택안: 빌드 상수

**근거**: Harvester와 동일한 원칙("budget 튜닝은 별도 change로만"). 운영 환경마다 값이 달라지면 재시작 사이클 분석이 복잡해진다. 100이 부적절한 것으로 판명되면 후속 change에서 상수 값을 한 번 바꾼다.

**주**: 단위 테스트 목적으로 package-private 필드(`budget int`)를 통해 소규모 값으로 override할 수 있는 경로는 존재할 수 있으나(구현 세부), 이는 env·config·CLI 노출이 아니며 외부 런타임 surface를 신설하지 않는다. 본 Decision의 "env 미노출" 원칙은 "런타임 config 경로 부재"를 의미한다.

### Decision 6: budget은 프로세스 로컬, 인메모리 카운터

**선택**: 복수 프로세스 환경에서 각 워커는 독립적으로 자신의 100회를 센다. DB/Redis 기반 공유 카운터는 도입하지 않는다.

**근거**: 카운터는 "이 프로세스의 수명 관리" 용도로, 프로세스 경계를 벗어날 이유가 없다. Harvester와 동일.

### Decision 7: idle 시나리오는 Dequeue 내부 책임

**선택**: frontier에 claim 가능한 URL이 없을 때는 `URLScheduler.Dequeue` 내부에서 1초 sleep 후 재시도한다(베이스라인 `specs/scheduler/spec.md`의 "폴링 주기 1초 고정" Scenario 참조). consumer(Pioneer 워커)는 별도 idle 처리를 하지 않으며, Dequeue가 URL을 반환할 때까지 대기하는 동안 **워커는 계속 살아 있다**. 이 대기 시간은 budget 카운트에 포함되지 않는다.

**근거**: idle 처리는 scheduler의 계약이므로 consumer가 중복 구현할 필요가 없다. 또한 idle 대기 중에도 카운터가 0으로 누적된 상태가 유지되므로 "장기 idle 워커가 영원히 산다"는 문제는 구조적으로 존재하지 않는다 — 워커가 할 일이 없으면 가만히 기다릴 뿐이고, 일이 들어오면 정확히 100회를 처리하고 종료한다.

### Decision 8: supervisor는 스펙 범위 밖

**선택**: 재시작 책임은 런타임 환경(systemd Restart=always, k8s Deployment restartPolicy, docker `restart: always` 등)에 위임. 스펙은 "워커는 종료한다"까지만 정의한다.

## Risks / Trade-offs

- **[Risk] 100회가 너무 작으면 재시작 오버헤드(프로세스 기동, scheduler 재연결, TLS handshake)가 처리량을 잠식** → 초기값 100으로 시작하고 로그/메트릭으로 재시작 빈도 관측 후 튜닝. 튜닝은 별도 change(상수 값 변경).
- **[Risk] 100회가 너무 커서 메모리 누수가 축적될 수 있음** → budget 도입 자체가 상한이므로 무한 성장 방지. 관측을 통해 조정.
- **[Risk] supervisor가 없는 환경에서 실행 시 워커가 단발성으로 끝남** → 운영 문서에 supervisor 필수 의존을 명시. 스펙도 "supervisor 책임"을 명문화.
- **[Risk] graceful 종료 중 진행 중 URL이 오래 걸리면 종료 지연** → 기존 Pioneer의 fetch 타임아웃이 이미 존재하므로 최악의 경우도 유한 시간 내 종료.
- **[Risk] 재시작 폭주** → 외부 사이트 장애로 fetch 실패가 빈발해도 Dequeue 자체는 성공하여 100회를 빠르게 소진할 수 있다. 이는 budget 정책 문제가 아니라 backoff 영역이며 `scheduler-retry-backoff`에서 다룬다.
- **[Trade-off] 메모리 누수를 진단·수정하는 대신 주기적 재시작으로 회피** → 근본 원인 해결은 아니지만, 실제 운영에서는 저비용·고신뢰 패턴. 누수가 식별되면 별도 fix.

## Migration Plan

1. 본 change 적용 시 Pioneer 워커 진입점에 성공 Dequeue 카운터·종료 분기를 추가한다.
2. 기존 배포 환경(docker-compose / k8s / systemd)에 재시작 정책이 설정되어 있는지 확인. 없으면 배포 가이드에 따라 supervisor 설정 추가 후 본 change 배포.
3. **Rollback**: budget 카운터·분기를 제거하면 **무한 루프로 복귀**한다. 데이터 측 마이그레이션 없음 (Harvester와 동일).

## Open Questions

- 100이라는 숫자 자체의 적정성은 운영 데이터 축적 후 재평가. 조정 시 빌드 상수 값만 변경(env 도입은 불가).
- supervisor가 SIGTERM을 보냈을 때 진행 중 fetch를 중단할 grace 기간은 supervisor 기본값에 일임. 필요 시 후속 change에서 명시.
