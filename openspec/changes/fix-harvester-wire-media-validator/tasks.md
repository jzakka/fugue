## 1. wiring 관찰 surface 추가

- [x] 1.1 `apps/api/internal/bot/harvester_consumer.go`의 `HarvesterConsumer`에 `HasMediaValidator() bool` 메서드를 추가한다. 단순 `return h.validator != nil`. design.md Decision 2 근거 주석 포함.
- [x] 1.2 단위 테스트(`harvester_consumer_test.go` 또는 신규 `harvester_consumer_wiring_test.go`)에 `HasMediaValidator()`의 두 경로(wiring 없음 → false, `WithMediaValidator` 호출 후 → true)를 검증하는 `func Test...` 테스트 케이스를 추가한다.

## 2. production 부트스트랩 wiring

- [x] 2.1 `apps/api/cmd/bot/` 패키지 안에 `buildHarvesterConsumer(...) *bot.HarvesterConsumer` 빌더 함수를 신설하여 `bot.NewHarvesterConsumer(...).WithMediaValidator(bot.NewDefaultMediaValidator())` 체이닝을 그 빌더에 담는다. design.md Decision 1.
- [x] 2.2 `apps/api/cmd/bot/main.go`의 harvester subcommand cobra `RunE` 클로저가 직접 `bot.NewHarvesterConsumer(...)`를 호출하던 부분을 2.1의 빌더 호출로 교체한다.
- [x] 2.3 빌더 정의 근처에 "spec: harvester `Harvester 워커 부트스트랩은 미디어 후보 유효성 검증기를 wire한다`" 한 줄 주석을 남겨 회귀 시 의도가 보이도록 한다.

## 3. wiring 회귀 테스트

- [x] 3.1 `apps/api/cmd/bot/`에 신규 테스트 파일(예: `harvester_wiring_test.go`)을 생성하고, 2.1의 `buildHarvesterConsumer(...)` 빌더를 호출하여 반환된 컨슈머의 `HasMediaValidator()`가 true임을 검증하는 `func Test...` 테스트를 추가한다.

## 4. 검증

- [x] 4.1 `cd apps/api && go build ./...` 통과.
- [x] 4.2 `cd apps/api && go test ./internal/bot/... ./cmd/bot/...` 통과.
- [x] 4.3 `openspec validate fix-harvester-wire-media-validator --strict` 통과.
