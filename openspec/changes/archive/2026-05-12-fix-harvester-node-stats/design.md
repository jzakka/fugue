## Context

Harvester는 한 노드(URL)를 처리할 때마다 결과를 5개 카운터(`PinsCreated`, `Deduped`, `Skipped`, `Failed`, `AdapterFallback`)로 분류해야 한다(spec.md "Harvester 노드 단위 통계 정의"). 현재 `HarvesterConsumer`는 fetch 실패 카운터(`fetchFailureCount`) 외에는 어떤 카운터도 보유하지 않는다. 운영팀은 노드 단위 통계를 외부 모니터링 시스템에 노출하기 위한 in-process 관찰점이 필요하고, 회귀 테스트는 분류 로직이 깨지지 않았음을 카운터로 검증할 수 있어야 한다.

`HarvestPipeline.ProcessDocument`는 이미 `created bool`을 반환하므로 신규 insert(`PinsCreated`)와 기존 update(`Deduped`)를 구분할 정보가 consumer까지 전달된다. `extractDocument`도 `fellBack bool`을 반환하므로 어댑터 실패 후 generic으로 fallback한 사실을 consumer가 안다.

## Goals / Non-Goals

**Goals:**
- 한 노드 처리당 주 카테고리(`PinsCreated`/`Deduped`/`Skipped`/`Failed`) 중 정확히 1개 카운터가 1 증가한다.
- 같은 노드에서 어댑터 실패가 발생하면 `AdapterFallback`이 추가로 1 증가한다.
- 카운터 값은 `HarvesterConsumer` 인스턴스에서 in-process로 관찰 가능하다.
- 워커 간 카운터 공유 없음(spec "Dequeue 카운터는 워커 간 공유 상태가 아니다"와 동일 정책 적용).

**Non-Goals:**
- 외부 메트릭 시스템(Prometheus 등) 연동. 본 변경은 in-process 카운터 노출까지만 책임진다.
- 카운터 영속화. 워커 종료 시 사라지는 것이 의도된 동작이다.
- 기존 `fetchFailureCount` 통합. 별개 의미를 가지므로 그대로 둔다.

## Decisions

### Decision 1: 단일 `NodeStats` 구조체로 5개 카운터를 묶는다

대안: 5개 필드를 `HarvesterConsumer`에 직접 둔다.

선택 이유: 카운터 묶음을 하나의 값으로 노출(`Stats() NodeStatsSnapshot`)하면 호출자가 다섯 번의 atomic load를 직접 조율하지 않아도 되고, 카운터가 늘어날 때 시그니처가 안정적이다. 다만 SnapShot은 원자적이지 않다(개별 카운터 atomic load의 모음)는 점은 운영 관찰성 수준에서 허용한다(스펙도 카운터 단위 정확성만 요구).

### Decision 2: 카운터는 `atomic.Uint64`

대안: `sync.Mutex` + `uint64`.

선택 이유: 카운터 증가는 핫 패스에서 단일 wirte이고 컨텐션이 없다. `fetchFailureCount`도 이미 `atomic.Uint64`이므로 일관성을 유지한다.

### Decision 3: 카운터 증가 지점은 `processOne` 종료 경로

대안: 각 헬퍼(`extractDocument`, `createPins`, `reportFailure`)가 카운터를 증가시킨다.

선택 이유: `processOne`에서 한 번에 분류 로직을 모으면 "노드 1개당 주 카테고리 정확히 1" 불변을 한 곳에서 강제할 수 있다. 헬퍼가 카운터를 만지면 분기 시 누락/중복 가능성이 생긴다.

증가 규칙:
- fetch 실패 → `Failed++` (+ 기존 `fetchFailureCount++` 유지)
- `extractDocument` 에러 → `Failed++`
- `fellBack == true` → `AdapterFallback++` (다른 주 카테고리와 독립적으로 증가)
- classifier가 `pinnable=false` → `Skipped++`
- `createPins` 에러 → `Failed++`
- 성공 + `created=true` → `PinsCreated++`
- 성공 + `created=false` → `Deduped++`

### Decision 4: `createPins`는 `created bool`을 함께 반환한다

기존 시그니처는 `([]uuid.UUID, error)`. `HarvestPipeline.ProcessDocument`가 `(created bool, pinID, err)`을 반환하므로 wrapping된 정보를 consumer까지 흘려주면 `PinsCreated`/`Deduped` 분기가 가능하다. 시그니처 변경 영향은 `createPins`가 `HarvesterConsumer` 내부 함수이므로 외부 노출 없음.

### Decision 5: 스펙 보강 - 관찰 가능성 시나리오 추가

현재 스펙은 카운터 의미론만 정의하고 관찰 수단을 명시하지 않아 "구현 안 됨"이 사실상 검증 불가능하다. 본 변경은 스펙에 "카운터 값이 워커 프로세스 lifetime 동안 관찰 가능하다" 시나리오를 추가한다. 구체 API명은 design 결정 사항으로 남기고 시나리오는 행위 계약 수준만 규정한다.

## Risks / Trade-offs

- [`NodeStatsSnapshot`의 비원자성으로 인한 운영 혼동] → Mitigation: 스냅샷 시점이 비원자적임을 doc string에 명시한다. 노드 단위 카운터는 트렌드 관찰 용도이며 절대 정합성이 필요한 자료가 아니므로 비원자 스냅샷이 의미를 훼손하지 않는다.
- [`createPins` 시그니처 변경으로 인한 회귀] → Mitigation: 내부 함수이므로 영향이 consumer 내부에 국한된다. 단위 테스트로 두 분기를 모두 커버한다.
- [기존 `MockPipeline` 동작과의 불일치] → Mitigation: `MockPipeline`은 이미 `(created, pinID, err)` 시그니처를 따르므로 추가 변경 불필요.

## Migration Plan

본 변경은 외부 시그니처를 변경하지 않으므로 인프라 마이그레이션 단계가 없다. 배포 후 즉시 카운터가 활성화되며, 카운터를 사용하는 후속 메트릭 export 작업은 별도 change로 분리한다.

## Open Questions

없음.
