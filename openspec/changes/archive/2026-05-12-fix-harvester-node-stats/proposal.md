## Why

`openspec/specs/harvester/spec.md`의 "Harvester 노드 단위 통계 정의" Requirement는 5개 카운터(`PinsCreated`, `Deduped`, `Skipped`, `Failed`, `AdapterFallback`)를 노드(URL)당 정확히 집계할 것을 요구한다. 그러나 현재 `apps/api/internal/bot/harvester_consumer.go`의 `HarvesterConsumer`는 이 카운터들을 보유하거나 증가시키지 않는다(보유 카운터는 `fetchFailureCount` 단 한 개이며 이는 별개 목적). 운영 관찰성과 회귀 테스트 양쪽에서 노드 단위 통계가 확인 불가능한 상태다.

## What Changes

- HarvesterConsumer에 노드 단위 카운터를 도입해 `processOne` 종료 시점에 카테고리별로 정확히 1씩 증가하도록 한다(주 카테고리는 상호 배타, `AdapterFallback`은 부가).
- 카운터는 인프로세스에서 관찰 가능한 형태로 노출하여(`atomic.Uint64` 기반) 회귀 테스트와 런타임 모니터링이 동일한 API를 사용한다.
- `fellBack` 분기가 카운터 증가 호출 사이트가 되도록 흐름을 정돈한다(현재는 로그만 남김).

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `harvester`: 기존 "Harvester 노드 단위 통계 정의" Requirement에 카운터 관찰 가능성 시나리오를 보강한다. 카운터 값 자체와 카테고리 의미론은 기존 시나리오를 그대로 유지한다.

## Impact

- 영향 코드: `apps/api/internal/bot/harvester_consumer.go`
- 영향 테스트: `apps/api/internal/bot/harvester_consumer_test.go`
- API 변경 없음. 외부 인터페이스(스키마/HTTP)는 변경하지 않는다.
- 신규 의존성 없음.
