## Context

scheduler capability의 `HostRateLimiter`(`apps/api/internal/scheduler/host_rate_limiter.go`)는 생성자 시점에 받은 세 인자(`defaultRatePerSec`, `defaultBurst`, `enabled`)로 동작하도록 이미 완성되어 있다. 단위 테스트(`host_rate_limiter_test.go`)도 두 가지 운영자 기본값과 비활성화 시나리오를 모두 검증한다. 즉 capability 내부는 스펙을 만족한다.

문제는 봇 엔트리포인트(`apps/api/cmd/bot/main.go`)의 두 호출처가 운영자 설정과 무관하게 공장 기본값을 그대로 전달한다는 것이다.

```go
// L258 (harvesterCmd)
sched := scheduler.NewPGURLScheduler(infra.DB).
    WithRateLimiter(scheduler.NewHostRateLimiter(scheduler.FactoryDefaultRatePerSec, scheduler.FactoryDefaultBurst, true))

// L474 (runPioneerConsumer)
sched := scheduler.NewPGURLScheduler(infra.DB).
    WithRateLimiter(scheduler.NewHostRateLimiter(scheduler.FactoryDefaultRatePerSec, scheduler.FactoryDefaultBurst, true))
```

`config.Load()`가 정의·로딩하는 `Config.SchedulerHostDefaultRatePerSec/Burst/Enabled`는 `cmd/server`(API 서버)에서는 호출되지만, `cmd/bot`은 `envOrDefault`만 사용하고 `config.Load()`를 한 번도 호출하지 않는다.

## Goals / Non-Goals

**Goals:**

- 봇 워커 두 명령(`pioneer`, `harvester`) 모두에서 host rate limiter 생성에 운영자 설정값이 도달한다.
- 와이어링 회귀를 방지하는 단위 테스트가 존재한다.
- 환경변수 미지정 시의 default 동작(rate=1, burst=5, enabled=true)은 비트 단위로 보존한다.

**Non-Goals:**

- 새 환경변수를 추가하거나 기존 변수의 의미를 바꾸지 않는다.
- `HostRateLimiter` 내부 로직, `Config` 구조체 필드, env 키 이름은 손대지 않는다.
- API 서버(`cmd/server`)의 와이어링은 본 변경 범위 밖이다(현재 API 서버는 scheduler를 직접 쓰지 않으므로 영향 없음).
- robots.txt가 호스트별 rate를 override하는 경로(`SetHostRate`)는 이미 spec을 만족하므로 손대지 않는다.

## Decisions

### Decision 1: `Infrastructure` 구조체에 `Config` 포인터를 추가한다

`initInfrastructure()`에서 `config.Load()`를 1회 호출하고 결과를 `Infrastructure.Config *config.Config` 필드로 노출한다. 두 명령은 `infra.Config.SchedulerHostDefaultRatePerSec` 등으로 동일 인스턴스를 참조한다.

**대안 1: 두 명령이 각자 `config.Load()`를 호출한다** — 환경변수가 프로세스 시작 동안 불변이므로 동일 결과를 만들지만 호출이 중복되고, 향후 다른 설정값도 사용할 때 같은 중복이 누적된다. 인프라 초기화 시 1회만 부른다는 단일 책임이 더 깔끔하다.

**대안 2: `config.Load()` 대신 봇 main에서 새 helper(`loadSchedulerHostConfig`)를 정의한다** — 봇 빌드 시 `config` 패키지 의존을 피할 수 있지만, 이미 server는 `config` 패키지를 빌드하므로 의존 추가 비용이 사실상 없다. 게다가 env 키 fallback 값을 두 군데에 적는 위험이 생긴다.

채택: 대안 0(메인 안). 한 번 부르고 인프라에 매단다.

### Decision 2: 와이어링 검증 테스트는 `cmd/bot` 패키지 내부의 단위 테스트로 작성한다

테스트 대상은 "Config 값이 host rate limiter 인자로 전달된다" 한 가지뿐이다. 풀 e2e 부팅 없이 검증하려면 와이어링 로직을 작은 함수로 추출해 (`buildHostRateLimiter(cfg *config.Config) *scheduler.HostRateLimiter` 정도) 외부에서 호출 가능하게 만들고, 단위 테스트가 그 함수에 다양한 Config를 주입해 결과 limiter의 동작을 검증한다.

**HostRateLimiter 동작 검증 방법:**

- `SetHostRate`로 host를 미리 등록하지 않은 채 `Allow(host)`를 호출하면 lazy 생성된 bucket이 사용된다.
- 운영자 기본값이 0.5/3인 경우: 동일 host로 burst 3 + 1 = 4회 연속 Allow를 호출하면 4번째는 false. 공장 기본값 1/5와 구분된다.
- `enabled=false`인 경우: 임의 host로 100회 Allow가 모두 true이며 limiter 내부 상태에 host map 항목이 생성되지 않는다.

이미 `host_rate_limiter_test.go`가 동등한 시나리오를 한 단계 더 아래에서 검증하지만, 봇 와이어링 회귀를 잡기 위해 cmd/bot 레벨에서도 검증을 둔다.

**대안: integration 테스트(실제 cobra 명령 실행)** — 너무 무겁고 DB/S3 의존 때문에 단위 테스트 격리가 어렵다. 거부.

### Decision 3: `config.Load()` 호출 실패 시 `initInfrastructure()`도 실패한다

`config.Load()`는 JWT_SECRET·OAuth 환경변수가 없으면 에러를 반환한다. 봇 명령은 그동안 이 값을 요구하지 않았으므로 무조건 `config.Load()`를 호출하면 새 의무 의존성이 생긴다.

**채택: 봇 main에서는 "scheduler host config만" 추출하는 좁은 헬퍼를 사용한다.** `config.Load()`를 그대로 부르지 않고, `config` 패키지가 별도로 export하는 `envFloat`/`envInt`/`envBool`만 호출하거나, 본 변경에서 scheduler host 전용 작은 loader를 노출한다. 이렇게 하면 봇이 OAuth 값 없이도 부팅할 수 있던 기존 운영성을 보존한다.

구체 안: `config` 패키지에 신규 export 함수 `LoadSchedulerHostConfig() SchedulerHostConfig` 를 추가하고, Config 구조체의 동일 필드는 그대로 둔 채 양쪽이 같은 env 키와 같은 default 상수를 공유하도록 한다. 봇 main은 `config.LoadSchedulerHostConfig()`만 호출한다.

**대안: 봇 main 내부에 인라인 env 파싱** — config 패키지가 SSoT가 되지 않아 server와 봇이 따로 env 키 이름을 갖게 될 위험. 거부.

## Risks / Trade-offs

- [환경변수 미지정 시 기본값 일치] → `envFloat("...", 1.0)`/`envInt("...", 5)`/`envBool("...", true)`의 fallback과 `FactoryDefaultRatePerSec=1.0`/`FactoryDefaultBurst=5`/하드코딩 `true`가 비트 단위로 일치함을 단위 테스트로 잠근다.
- [`config` 패키지 분기 추가 위험] → 신규 export 함수는 기존 Config 구조체와 같은 env 키·같은 default를 사용한다. `Load()`는 손대지 않는다.
- [추출된 와이어링 헬퍼의 호출처가 늘어남] → 두 명령 모두 같은 헬퍼를 호출하도록 강제해 두 군데 동기화를 유지한다.

## Migration Plan

- 운영 환경 변경 없음. 환경변수를 지정하지 않은 환경에서는 기존과 동일하게 동작한다.
- 변경된 와이어링은 봇 워커 재기동 시 즉시 발효된다.
- 롤백: 본 PR을 revert하면 와이어링이 공장 기본값 하드코딩으로 돌아간다.

## Open Questions

없음.
