## Context

Harvester는 `harvester-scheduler-consumer` change 이후 `URLScheduler.Dequeue`를 폴링하며 각 URL에 대해 콘텐츠 추출과 Pin 생성을 수행하는 워커 프로세스로 운영된다. 워커는 복수 인스턴스로 떠 있을 수 있고, 각 인스턴스는 자체 Goja VM 풀, HTTP 커넥션, 임시 파일을 누적 사용한다.

장기 실행 시 다음 문제가 알려져 있다:

- Goja Runtime의 누적 할당으로 RSS가 천천히 증가한다.
- 외부 사이트와의 keep-alive 커넥션, 임시 다운로드 파일 등의 FD 누수.
- harvest 스크립트(사용자 정의 JS)가 전역 상태를 의도치 않게 보존할 위험.

`pioneer-worker-budget`은 동일한 운영 이유로 Pioneer 워커의 수명을 "Dequeue 100회 후 종료"로 제한한다. 본 change는 같은 정책을 Harvester에도 적용한다.

## Goals / Non-Goals

**Goals:**
- Harvester 워커 프로세스의 수명을 결정적으로 제한한다(정확히 100회 Dequeue 후 종료).
- 종료가 진행 중 작업을 손상시키지 않도록 graceful shutdown 의미를 명시한다.
- 재시작 책임을 워커 외부(supervisor)로 분리하여 워커 코드를 단순하게 유지한다.

**Non-Goals:**
- Pioneer 워커의 budget 정책 변경(이미 `pioneer-worker-budget`에서 정의).
- `URLScheduler` 동작 변경(claim/ack/backoff 정책은 `scheduler-claim-api`, `scheduler-retry-backoff` 범위).
- Harvester 콘텐츠 추출 파이프라인, Pin 생성 트랜잭션, 이미지 캐시 정책.
- supervisor 자체 구성(systemd unit, k8s manifest, docker compose 정책)은 운영 문서 범위.
- budget 값(100)을 동적으로 변경하는 설정·환경변수.

## Decisions

### Decision 1: budget 값은 100 (Pioneer와 동일, 빌드 상수, env 노출 금지)

**선택**: Harvester 워커도 Dequeue를 정확히 100회 수행한 뒤 종료한다. 이 값은 **빌드 타임 상수**로 고정하며, 환경변수·설정 파일·CLI 플래그로 노출하지 않는다(DECISIONS §6). Pioneer와 동일한 정책으로 운영자 멘탈 모델을 단일화한다.

**대안**:
- (A) Pioneer와 다른 값(예: 50, 200)
- (B) 시간 기반 종료(예: 15분 후 종료)
- (C) 메모리 임계치 기반 종료
- (D) 본 change 채택안: Pioneer와 동일한 100회

**근거**:
- 운영자 관점에서 "워커 프로세스는 100회 일하고 죽는다"는 단일 멘탈 모델이 가장 단순.
- 시간/메모리 기반은 측정·튜닝 변수가 더 들어가고, 짧은 작업과 긴 작업이 섞일 때 예측이 어렵다.
- 실제 100이 너무 작거나 큰 경우, 후속 change에서 한 번만 조정하면 된다(현 단계에서 설정 가능하게 만들 가치가 적다).

### Decision 2: 100회째 Dequeue 작업은 끝까지 수행 (graceful)

**선택**: 100회째 Dequeue로 받은 URL의 harvest 작업은 중단하지 않고 완료한 뒤 프로세스를 종료한다. Dequeue 카운터는 **성공 Dequeue 직후**(= URL을 실제로 반환한 호출이 리턴된 직후) 1 증가시키며, 카운터가 100에 도달하면 해당 URL의 harvest 파이프라인을 끝까지 수행한 뒤 루프를 빠져나온다. 특히 **100회째 harvest 작업이 `harvester_frontier` 갱신 및 `harvester_frontier_pins` INSERT(트랜잭션 커밋)까지 완료된 이후**에만 exit 0으로 종료한다.

빈 Dequeue·오류 Dequeue는 카운트하지 않는다. `URLScheduler.Dequeue`는 `scheduler-claim-api` 규약상 내부 blocking이므로 consumer 레벨에서는 원칙적으로 성공 반환만 보이지만, 스케줄러 자체 오류(DB 실패 등)로 에러를 반환할 여지가 있다. 두 경우 모두 카운터를 건드리지 않는다(아래 §Open Questions 닫힘 참조).

**대안**:
- (A) 100회째 Dequeue 직후 즉시 종료(harvest 결과 폐기)
- (B) 본 change 채택안: 100회째 작업까지 완료 후 종료
- (C) 100회째 작업의 외부 요청 timeout을 강제로 줄여 빠르게 종료

**근거**:
- 진행 중 작업을 폐기하면 frontier가 "claim된 채로 timeout 대기" 상태가 되어 다른 워커가 같은 URL을 다시 잡기까지 지연이 생긴다.
- harvest 단위 작업은 일반적으로 짧고(단건 페이지 처리), 한 작업을 끝내는 비용보다 partial 작업의 정합성 비용이 크다.
- frontier 갱신과 pin INSERT가 단일 트랜잭션으로 묶여 있으므로, 그 커밋 직후가 가장 안전한 종료 지점이다.
- supervisor가 SIGTERM을 보낼 때를 대비한 별도의 강제 종료 경로(예: SIGKILL grace period)는 supervisor 책임으로 둔다.

### Decision 3: 종료 코드는 0 (정상 종료)

**선택**: budget 소진은 비정상 상태가 아니므로 exit code 0으로 종료한다. supervisor는 "정상 종료 시 즉시 재시작"으로 설정한다.

**대안**:
- (A) 별도 종료 코드(예: 64)로 "budget exhaustion" 의미 표현
- (B) 본 change 채택안: exit 0 + 종료 직전 명시적 로그

**근거**:
- exit code로 의미를 구분하면 supervisor 정책이 환경별로 분기되어 일관성이 떨어진다.
- "정상 종료 시 재시작" 정책은 systemd `Restart=always`, k8s `restartPolicy: Always` 등 모든 supervisor에서 기본 형태.
- 종료 사유 구분이 필요한 경우 로그(`reason=budget_exhausted`)로 충분하다.

### Decision 4: Dequeue 카운터는 인메모리 변수 (공유 금지)

**선택**: 카운터는 워커 프로세스의 로컬 변수(예: `for i := 0; i < 100; i++`)로 보관한다. DB나 Redis에 저장하지 않는다.

**대안**:
- (A) frontier 또는 별도 테이블에 워커 단위 카운터 저장
- (B) 본 change 채택안: 인메모리 변수만 사용

**근거**:
- 카운터는 "이 프로세스의 수명 관리" 용도이므로 프로세스 경계를 벗어날 이유가 없다.
- 복수 워커가 공유해야 하는 상태(어떤 URL을 누가 처리 중인지 등)는 frontier에 이미 보관된다. 본 카운터는 그와 무관.
- "복수 프로세스에서 인메모리 상태 금지" 원칙은 "워커 간 공유가 필요한 도메인 상태"에 대한 제약이다. 자기 수명 카운터는 그 제약 대상이 아니다.

### Decision 5: 재시작은 supervisor 책임

**선택**: 워커 프로세스 자체에는 재기동 로직(자기 자신을 다시 fork/exec)을 두지 않는다.

**대안**:
- (A) 워커가 종료 직전 자식 프로세스를 spawn
- (B) 본 change 채택안: supervisor에 일임

**근거**:
- 워커가 자신을 재기동하면 supervisor 모니터링·로깅과 충돌하고, 누수가 누적된 부모 메모리를 자식이 일부 상속할 수 있다(fork 방식).
- 모든 운영 환경(systemd, k8s, docker, foreman)에 이미 재시작 메커니즘이 있다.

## Risks / Trade-offs

- **supervisor 미배포 환경에서 워커가 멈춤** → 운영 가이드(`docs/architecture.md` 또는 README)에 "Harvester는 supervisor 하에서 실행해야 한다"를 명시하고, docker-compose 예시에 `restart: always`를 포함한다. 로컬 개발에서는 단순 셸 루프(`while true; do harvester; done`)로 대체 가능.
- **재시작 폭주** → 외부 사이트 장애로 모든 Dequeue가 빠르게 실패하면 100회를 분 단위로 소진하고 재시작이 빈발할 수 있다. 이는 budget 정책 자체의 문제가 아니라 backoff 부재의 문제이며, `scheduler-retry-backoff`에서 다룬다. 본 change는 워커 수명만 정의.
- **빈 Dequeue도 카운터에 포함되는지 불명확** → "성공적으로 URL을 받은 Dequeue만 카운트"하도록 spec에서 명시한다. `URLScheduler.Dequeue`는 내부 blocking이므로 빈 큐 상황은 스케줄러 내부에서 sleep되어 consumer에는 드러나지 않지만, 방어적으로 "URL이 리턴되지 않은 호출은 카운트하지 않는다"를 명문화한다. 이로써 idle 상태에서 빠른 종료/재시작 폭주를 방지.
- **Dequeue 오류 시 카운트 여부** → "오류 반환 시에도 미카운트". 스케줄러 자체 오류(DB 실패 등)로 `Dequeue`가 에러를 반환한 경우 카운터를 증가시키지 않고 로깅 후 재시도한다. 오류 반복으로 인한 무한 루프 방지는 `scheduler-retry-backoff` 및 supervisor 레벨 backoff로 다룬다. (이 항목은 과거 Open Question을 본 change에서 종결.)
- **100회 도중 panic** → graceful 정의는 정상 흐름에 한정. panic은 supervisor가 비정상 종료로 처리하여 재시작. 본 change 범위 외.

## Migration Plan

1. 본 change 적용 시 Harvester 워커 진입점에 카운터·종료 분기를 추가한다.
2. 기존 배포 환경에 supervisor 재시작 정책이 설정되어 있는지 확인. 없는 경우 배포 가이드에 따라 supervisor 설정 추가 후 본 change 배포.
3. 롤백: budget 분기(카운터·break 조건)를 제거하면 **무한 Dequeue 루프로 복귀**한다(기존 동작). 데이터 측 마이그레이션·스키마 변경 없음. 롤백은 코드 revert만으로 가능하다.

## Open Questions

- ~~"성공적으로 URL을 받은 Dequeue"의 정의 — Dequeue가 에러를 반환한 경우(스케줄러 자체 오류)는 카운트할지~~ → **종결**. 오류 반환 시에도 미카운트(위 Risks/Trade-offs 참조). DECISIONS §6 및 `scheduler-claim-api`와 정합.
- supervisor가 SIGTERM을 보냈을 때 진행 중 harvest를 중단할 grace 기간(현재는 supervisor 기본값에 일임). harvest 단건 처리 시간 분포가 측정되면 가이드 값을 명시.
