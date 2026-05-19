## 1. Surface 분리

- [x] 1.1 `apps/api/cmd/bot/pioneer_consumer_builder.go` 신규 작성: `buildPioneerConsumer(sched scheduler.URLScheduler, store snapshot.SnapshotStore, rl bot.HostRateSetter) (*bot.PioneerConsumer, *bot.RobotsFilter)` 시그니처. 내부에서 `bot.NewRobotsFilter(rl)` 인스턴스를 변수로 보관해 FilterChain에 끼우는 동시에 결정적 wiring 검증을 위해 반환에도 포함. seed enqueue·infra wiring은 호출자(`runPioneerConsumer`)가 책임.

## 2. Wiring 교체

- [x] 2.1 `apps/api/cmd/bot/main.go`의 `runPioneerConsumer`에서 `buildHostRateLimiter(config.LoadSchedulerHostConfig())`를 지역 변수 `rl`로 추출.
- [x] 2.2 sched 생성 시 `WithRateLimiter(rl)` 사용. `buildPioneerConsumer(sched, store, rl)` 호출로 PioneerConsumer 조립을 위임. `bot.NewRobotsFilter(nil)` 직접 호출 제거.

## 3. 테스트

- [x] 3.1 `apps/api/cmd/bot/pioneer_consumer_builder_test.go` 신규: fake `HostRateSetter`(`SetHostRate` 호출을 record하는 mock)을 `buildPioneerConsumer`에 전달한 뒤, 반환된 RobotsFilter의 `RateSetter()`가 fake와 정확히 동일 인스턴스이고, 그 setter로 `SetHostRate`를 호출하면 fake에 record되는지 결정적으로 검증. 보조 테스트로 RateSetter가 nil이 아님을 가드.
- [x] 3.2 `go test ./apps/api/cmd/bot/...` 통과 확인.
- [x] 3.3 `go test ./apps/api/internal/bot/...` 통과 확인(회귀 없음).

## 4. Spec/문서

- [x] 4.1 `openspec/changes/fix-pioneer-robots-filter-host-rate-setter-wiring/specs/bot/spec.md` 작성: ADDED Requirement "Pioneer 부트스트랩은 RobotsFilter에 HostRateLimiter를 wire한다"와 Scenario 2개.
- [x] 4.2 wiring 검증을 결정적으로 가능케 하기 위해 `apps/api/internal/bot/robots_filter.go`에 read-only accessor `RateSetter() HostRateSetter` 1개 추가(design Non-Goals의 "시그니처/인터페이스 미변경"과 충돌 없음).

## 5. 정합성 검증

- [x] 5.1 `go build ./apps/api/...` 통과 확인.
- [x] 5.2 진행 중 change `fix-scheduler-host-rate-limiter-config-wiring`와의 코드 충돌 없음 확인(같은 함수의 다른 라인을 만지며, 본 change 적용 시점에 그 change는 이미 tasks 전부 완료 상태).
