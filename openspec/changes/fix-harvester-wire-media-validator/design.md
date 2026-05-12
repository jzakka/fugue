## Context

`openspec/specs/harvester/spec.md`의 4개 미디어 검증 Requirement는 SHALL 계약이지만 production harvester 워커에서 실제로 enforce되지 않는다. 검증 자체의 구현(`DefaultMediaValidator`, `FilterValidMedia`)과 컨슈머의 옵트인 surface(`HarvesterConsumer.WithMediaValidator`)는 archive된 `2026-04-27-harvester-media-validation` change에서 도입되었지만, `apps/api/cmd/bot/main.go`의 harvester subcommand 부트스트랩이 `WithMediaValidator(...)`를 체이닝하지 않아 `HarvesterConsumer.validator` 필드가 nil 상태로 production에 배포되어 왔다. 그 결과 `harvester_consumer.go`의 `if h.validator != nil { FilterValidMedia(...) }` 분기가 항상 우회되고, 무효 미디어가 spec 위반 상태로 Pin의 정본 미디어로 채택된다.

본 변경은 그 wiring 누락만 닫는다. 행위 자체의 정의는 기존 4개 Requirement에서 이미 SHALL로 명시되어 있으므로 본 change는 그 enforcement 경로만 보장하는 추가 Requirement 하나를 도입한다.

## Goals / Non-Goals

**Goals:**
- production harvester subcommand 부트스트랩이 HarvesterConsumer에 MediaValidator를 wiring한다 (SHALL).
- production wiring이 외부에서 관찰 가능하게 하여 단위 테스트가 wiring 누락을 회귀로 잡을 수 있다.
- 기존 4개 미디어 검증 Requirement의 SHALL 본문은 변경하지 않는다.

**Non-Goals:**
- `DefaultMediaValidator`의 검증 임계값 또는 알고리즘 변경.
- Fetcher의 3-tuple `(html, errorKind, err)` 계약 갭 해소 (별도 Requirement, 별도 change).
- 배포 이전 누적 Pin의 backfill (Requirement 단서에 따라 운영 작업으로 위임).
- ValidationMetrics 싱크 wiring (별도 운영 결정; 본 change는 검증 자체의 enforcement만 보장).
- `cmd/pioneer` 또는 다른 부트스트랩 경로 변경 (본 change는 harvester subcommand만 다룬다).

## Decisions

### Decision 1: 부트스트랩 wiring은 `cmd/bot` 패키지 내부의 빌더 함수로 추출한다

**선택**: harvester subcommand의 cobra `RunE` 클로저 안에서 직접 체이닝하는 대신, `cmd/bot` 패키지 안에 작은 빌더 함수 — 예: `buildHarvesterConsumer(...) *bot.HarvesterConsumer` — 를 추가하고 거기서 `bot.NewHarvesterConsumer(...).WithMediaValidator(bot.NewDefaultMediaValidator())`를 체이닝한다. cobra RunE은 이 빌더를 호출하기만 한다.

**대안**:
- (a) `NewHarvesterConsumer` 생성자가 기본 validator를 자동 wiring하게 변경 → 기존 컨슈머 테스트(`harvester_consumer_test.go`)가 validator 우회를 전제로 작성되어 있어 광범위한 테스트 수정 유발.
- (b) cobra `RunE` 클로저 안에서 직접 체이닝 → 회귀 테스트가 그 클로저를 직접 호출할 수 없어 wiring을 외부에서 관찰할 surface가 사라진다.
- (c) 빌더를 `internal/bot`에 신설 → 라이브러리 surface를 확장하므로 본 change 범위 초과.

**근거**: (b)는 wiring 누락 회귀 방어를 코드 surface에서 확보할 수 없어 spec의 두 번째 SHALL("외부에서 결정적으로 관찰 가능")을 충족하기 어렵다. (a)는 컨슈머 기본 행위 변경으로 SDK 사용자(테스트 포함)에게 silent 영향을 준다. `cmd/bot` 패키지 내부 빌더 추출은 라이브러리 surface 확장 없이 회귀 테스트가 wiring을 직접 검증할 수 있게 한다. 본 change의 핵심 갭(부트스트랩 wiring 누락)이 정확히 부트스트랩 패키지 안에 있으므로 추출 비용도 최소다.

### Decision 2: wiring 검증은 `HarvesterConsumer`에 `HasMediaValidator() bool` 접근자를 추가해서 노출한다

**선택**: `HarvesterConsumer`에 `func (h *HarvesterConsumer) HasMediaValidator() bool { return h.validator != nil }` 접근자를 추가한다. 부트스트랩 단위 테스트는 이 접근자로 wiring을 검증한다.

**대안**:
- (a) Decision 1의 빌더만으로 검증하기 위해, 빌더 반환값의 `validator` 비공개 필드를 같은 `cmd/bot` 패키지 안의 테스트에서 직접 비교 → 비공개 필드 접근에 테스트가 결합되어, 후속 리팩터링(필드 이름 변경, 컨슈머 내부 구조 변경)에 취약하다. 또한 `internal/bot` 외부에서 wiring 사실을 관찰할 surface가 없어, 다른 부트스트랩 경로가 생겼을 때 같은 회귀 방어가 적용되지 않는다.
- (b) 행위 통합 테스트로만 검증(실제 무효 미디어 URL을 처리해 PinDocument 결과가 비어 있음을 확인) → 부트스트랩 wiring 자체를 직접 검증하지 않으며, 실패가 와이어링 누락인지 다른 회귀인지 진단이 어렵다.

**근거**: 접근자는 1줄짜리 surface로, "validator가 wire되어 있다"는 외부 관찰 가능한 사실만 노출한다. 검증 동작/임계값은 노출하지 않으므로 행위 계약을 누설하지 않는다. (a)의 비공개 필드 접근 방식은 빌더 추출(Decision 1)과 결합되어도 회귀 방어가 부트스트랩 패키지 내부로 한정되므로 채택하지 않는다.

### Decision 3: spec delta는 "production 부트스트랩은 MediaValidator를 wire한다" Requirement 1건만 추가한다

**선택**: harvester capability spec에 다음 한 개 Requirement를 추가한다 (개요):
- Requirement: Harvester 워커 부트스트랩은 미디어 후보 유효성 검증기를 wire한다 — production 부트스트랩이 구성하는 HarvesterConsumer는 미디어 후보 유효성 검증기를 보유한 상태여야 한다(SHALL). 외부 코드가 내부 필드 접근 없이 wiring 상태를 결정적으로 관찰할 수 있어야 한다(SHALL).

**대안**:
- (a) spec에 새 Requirement 추가하지 않고 기존 4개의 enforcement만 코드로 닫는다 → 회귀 방지 surface가 spec 수준에 없으므로 누군가 다시 wiring을 빼도 spec status가 그대로 통과한다.

**근거**: enforcement 갭이 발생했다는 사실 자체가 회귀 가능성의 증거다. spec 수준에서 "production은 wire한다"를 SHALL로 못박으면 위반이 spec 단계에서 탐지된다. 새 Requirement는 행위가 아닌 "wiring 사실"의 관찰 가능성만 규정하므로 기존 4개 Requirement의 SHALL 본문을 침범하지 않는다.

## Risks / Trade-offs

- **[Risk] validator wiring으로 인한 신규 처리 부담**: 모든 노드가 ffprobe/HTTP HEAD를 추가로 호출 → harvester 처리 시간 증가, ffprobe 의존성이 컨테이너에 누락된 환경에서 video/audio 검증 실패. **Mitigation**: `apps/api/Dockerfile`에 ffmpeg가 포함되어 있음(CLAUDE.md 명시). 로컬 개발 환경은 brew/apt로 ffmpeg를 갖추어야 한다. image 검증은 stdlib만 사용하므로 의존성 영향 없음.
- **[Risk] no_primary_media 분류 증가로 Pin 생성률 단기 하락**: 기존에 잘못 통과했던 무효 미디어 페이지가 이제 정상 거부된다. **Mitigation**: spec의 의도된 행위이며, classifier reason이 `no_primary_media`로 기록되어 운영 가시성이 보장된다.
- **[Trade-off] HasMediaValidator() 접근자 노출**: 내부 상태(validator field nil 여부)가 surface에 새 method로 드러난다. 이는 wiring 검증을 위한 최소 surface이며 검증 동작 자체는 노출하지 않는다.
- **[Trade-off] Decision 1의 빌더 추출 방식**: `cmd/bot` 패키지에 부트스트랩 wiring 전용 빌더 함수가 추가되어 패키지 surface가 한 함수만큼 늘어난다. 추가 부트스트랩 경로(예: pioneer subcommand 등)가 생길 경우 동일 빌더 재사용 또는 별도 빌더 신설은 그 시점에 결정한다.
