## Context

`openspec/specs/harvester/spec.md`의 `Harvester 노드 단위 통계 정의` Requirement는 `AdapterFallback`을 "주 카테고리와 독립적인 부가 카운터이며 주 카테고리와 동시에 증가할 수 있다(SHALL)"로 규정한다. 동일 Requirement의 명시적 Scenario "어댑터 실패 후 generic 성공"은 generic이 성공해 `PinsCreated|Deduped`가 증가하는 동시에 `AdapterFallback`이 1 증가함을 요구한다. SHALL 본문의 "주 카테고리와 동시에 증가할 수 있다"는 어댑터 실패라는 사실이 발생했을 때 generic의 결과(`PinsCreated|Deduped|Failed`)와 무관하게 부가 카운터가 함께 증가함을 함의한다.

production 코드는 `processOne`이 `extractDocument` 결과를 받은 직후 `extractErr != nil`이면 `Failed`만 증가시키고 조기 반환한다. 그 뒤에 위치한 `if fellBack { adapterFallback.Add(1) }` 분기는 generic이 성공한 경우에만 도달한다. 결과: 어댑터 실패+generic 실패 경로에서 `AdapterFallback`이 영구히 0으로 유지된다. 본 change는 그 분기 누락만 닫는다.

## Goals / Non-Goals

**Goals:**
- 어댑터 실패가 발생한 모든 노드 처리에서 `AdapterFallback` 카운터가 1 증가한다.
- 어댑터 실패+generic 실패 경로에서는 `Failed`와 `AdapterFallback`이 동시에 증가한다.
- 기존 SHALL "주 카테고리는 노드당 정확히 1"은 그대로 유지된다.

**Non-Goals:**
- 5개 카운터 외 새 카운터 도입.
- `NodeStats` 관찰 surface 변경.
- adapter/generic extractor 알고리즘 변경.
- adapter 실패의 원인(타입) 세분화.

## Decisions

### Decision 1: `if fellBack` 처리를 `extractErr` 조기 반환보다 앞으로 옮긴다

**선택**: `processOne`에서 `doc, fellBack, extractErr := h.extractDocument(...)` 호출 직후, `extractErr` 검사보다 먼저 `if fellBack { h.stats.adapterFallback.Add(1) }`를 실행한다. 그 뒤 `extractErr != nil`이면 기존대로 `Failed` 증가 + 조기 반환.

**대안**:
- (a) `extractErr != nil` 분기 안에 `if fellBack { adapterFallback.Add(1) }`를 복제 → 동일 카운터 증가 코드가 두 곳에 분산되어 회귀 시 한 쪽만 빠질 위험.
- (b) `extractDocument` 내부에서 `adapterFallback` 카운터를 증가시킨다 → 통계 집계가 호출자(`processOne`)에 모여 있는 현재 구조와 어긋난다(다른 카운터는 모두 `processOne`에서 증가).

**근거**: 부가 카운터의 의미가 "어댑터 fallback 시도가 일어났다"이므로, 그 fact가 관찰된 시점(=`fellBack==true`을 받은 직후)에 1회 증가시키는 것이 호출 흐름과 일치한다. 단일 호출 지점이 되어 회귀 방어도 단순해진다.

### Decision 2: 새 Scenario를 spec delta에 추가하여 회귀를 spec 수준에서 차단한다

**선택**: harvester capability spec의 `Harvester 노드 단위 통계 정의` Requirement에 다음 한 개 Scenario를 추가한다 (개요):
- Scenario: 어댑터 실패 후 generic 실패 — WHEN ScriptAdapter가 실패하고 fallback된 generic extractor도 실패할 때, THEN Failed가 1 증가하고 별도로 AdapterFallback이 1 증가한다.

**대안**:
- (a) Scenario 추가 없이 코드만 수정 → SHALL 본문 "동시에 증가할 수 있다"의 적용 범위가 모호하게 남고, 누군가 다시 분기를 회귀시켜도 spec validate가 통과한다.

**근거**: 기존 Scenario 목록이 "어댑터 실패 후 generic 성공"만 다루고 있어 "어댑터 실패 후 generic 실패" 경로의 회귀가 spec 단계에서 잡히지 않는다. 새 Scenario는 SHALL 본문을 변경하지 않고 기존 SHALL의 한 적용 케이스를 명시적으로 노출한다.

### Decision 3: 회귀 테스트는 기존 `withExtractor` seam과 mock `genericExtractorIface`로 generic 실패를 주입한다

**선택**: `harvester_consumer.go`의 기존 `genericExtractorIface` 인터페이스(line 44)와 패키지 내부 `withExtractor` 메서드(line 223)를 사용한다. 회귀 테스트는 `Extract`이 항상 에러를 반환하는 stub을 만들고 `withExtractor`로 주입한 뒤, 기존 `failingAdapter`(테스트 헬퍼)와 결합하여 "fellBack=true & err!=nil" 경로를 결정적으로 재현한다.

**대안**:
- (a) `HarvesterConsumer`에 새 unexported `extractFn` 함수 필드를 도입한다 → 이미 존재하는 `genericExtractorIface` + `withExtractor` seam과 동일한 회귀 방어를 더 큰 surface 변경으로 제공하므로 잉여.
- (b) 회귀 테스트 없이 코드만 수정한다 → spec의 새 Scenario가 코드 수준에서 enforce되지 않아 회귀 시 spec validate만으로는 차단할 수 없다.

**근거**: `harvester_consumer.go` line 39-46이 명시적으로 "parse-failure tests can inject a stub that errors — the real GenericExtractor is defensive and almost never returns an error"라고 적시한다. 즉 본 회귀 테스트가 필요로 하는 seam이 이미 같은 의도로 도입되어 있다. 새로운 hook을 추가하지 않고 기존 seam을 사용하면 본 change의 코드 변경 범위가 분기 1개 이동(Decision 1)으로 최소화된다.

## Risks / Trade-offs

- **[Risk] AdapterFallback 카운터 단기 증가**: 어댑터 실패+generic 실패 경로가 실제로 관찰되어 온 사이트에서 `AdapterFallback` 누적값이 단기 상승한다. **Mitigation**: 의도된 행위이며 `Failed` 값은 영향받지 않으므로 알람/대시보드 임계값이 `Failed` 기반이면 영향 없다.
- **[Trade-off] processOne의 처리 순서 변경**: `extractErr` 처리 직전에 `if fellBack` 한 줄이 끼어들어 코드 흐름의 작은 비대칭이 생긴다. 다만 변경은 5 LOC 이하이며 주석으로 의도(부가 카운터 + 주 카테고리 동시 증가)가 명시되므로 가독성 영향은 미미하다.
