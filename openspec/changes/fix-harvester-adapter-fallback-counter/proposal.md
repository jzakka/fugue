## Why

`openspec/specs/harvester/spec.md`의 `Harvester 노드 단위 통계 정의` Requirement는 "`AdapterFallback`은 주 카테고리와 독립적인 부가 카운터이며 주 카테고리와 동시에 증가할 수 있다(SHALL)"라고 명시한다. 그러나 production HarvesterConsumer는 어댑터가 실패하고 generic extractor도 실패하는 경로에서 `AdapterFallback`을 증가시키지 않는다. `apps/api/internal/bot/harvester_consumer.go`의 `processOne`이 `extractErr != nil`인 경우 `Failed` 카운터만 증가시키고 조기 반환하기 때문에, 그 뒤에 있는 `if fellBack { adapterFallback.Add(1) }` 분기에 도달하지 못한다. 결과: 어댑터 실패가 일어났다는 사실이 `Failed`가 동시에 증가한 경우 외부 관찰에서 사라진다.

본 change는 그 분기 누락만 한정해서 닫는다. 어댑터 실패가 발생했다면 generic의 성공/실패와 무관하게 `AdapterFallback`이 증가하도록 한다.

## What Changes

- 어댑터가 실패한 노드 처리에서는 generic extractor의 성공/실패와 무관하게 `AdapterFallback` 카운터가 1 증가한다.
- 어댑터 실패와 generic extractor 실패가 모두 발생한 노드는 `Failed`와 `AdapterFallback`이 동시에 1씩 증가한다.

## Capabilities

### New Capabilities
<!-- 없음 -->

### Modified Capabilities

- `harvester`: 기존 Requirement `Harvester 노드 단위 통계 정의`의 본문 SHALL 3개와 기존 6개 Scenario는 유지하면서, 본문에 보강 SHALL 한 문장("어댑터 실패가 한 노드 처리에서 발생했다면 generic extractor의 성공/실패와 무관하게 `AdapterFallback`이 1 증가해야 한다")과 새 Scenario("어댑터 실패 후 generic 실패")를 추가한다(적용 후 SHALL 4 / Scenario 7). 새 SHALL은 기존 "주 카테고리와 동시에 증가할 수 있다" SHALL의 적용 범위를 명시화하는 것이며 기존 SHALL의 의미를 좁히거나 넓히지 않는다.

## Impact

- 영향 코드: `apps/api/internal/bot/harvester_consumer.go`의 `processOne` 함수.
- 운영 지표: 어댑터 실패+generic 실패 경로의 `AdapterFallback` 카운터가 정상 증가. `Failed` 카운터 값은 변하지 않는다.
- 의존성·인프라·DB 마이그레이션 없음.
