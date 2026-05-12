## Why

`openspec/specs/harvester/spec.md`의 4개 Requirement — "미디어 후보 유효성 검증", "정본 키 영속 제한", "검증 실패 사유의 og_data 기록", "Pin primary media invariant" — 는 모두 SHALL 계약이지만, production HarvesterConsumer가 MediaValidator를 wiring하지 않아 외부 관찰 가능한 행위로 enforce되지 않는다. `apps/api/internal/bot/media_validator.go`에 `DefaultMediaValidator`/`NewDefaultMediaValidator()`가 구현되어 있고 `HarvesterConsumer.WithMediaValidator(...)` 메서드도 존재하지만, `apps/api/cmd/bot/main.go`의 harvester subcommand 부트스트랩이 이를 호출하지 않아 `harvester_consumer.go`의 `FilterValidMedia` 호출 분기가 production에서 항상 우회된다. 그 결과 1x1 placeholder GIF 같은 무효 미디어가 그대로 Pin의 정본 미디어로 채택되어 SHALL이 위반된다. 본 change는 부트스트랩 wiring 누락만 한정해서 닫는다.

## What Changes

- production harvester 부트스트랩이 Harvester consumer를 구성할 때 미디어 후보 유효성 검증기를 wire한다.
- consumer가 미디어 후보 유효성 검증기를 보유했는지 외부 코드가 결정적으로 관찰할 수 있게 한다.

## Capabilities

### New Capabilities
<!-- 없음 -->

### Modified Capabilities
- `harvester`: 기존 4개 Requirement("미디어 후보 유효성 검증", "정본 키 영속 제한", "검증 실패 사유의 og_data 기록", "Pin primary media invariant")가 production 부트스트랩에서 enforce되도록, "Harvester worker 부트스트랩은 미디어 후보 유효성 검증기를 wire한다" 보조 Requirement를 추가한다. 기존 4개 Requirement의 SHALL 본문은 변경하지 않는다.

## Impact

- 영향 코드: `apps/api/cmd/bot/main.go`의 harvester subcommand 부트스트랩 및 `apps/api/internal/bot/harvester_consumer.go`(wiring 관찰 surface 추가).
- 운영 지표: 무효 미디어 후보가 제거되어 classifier의 `no_primary_media` reason 카운트가 단기 증가, Pin 신규 생성률은 일부 하락 가능(=spec의 의도된 행위).
- 의존성, 비-범위 갭, 누적 Pin backfill 정책은 `design.md`(Risks/Trade-offs, Non-Goals) 참조.
