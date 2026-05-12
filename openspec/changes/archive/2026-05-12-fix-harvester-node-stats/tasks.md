## 1. NodeStats 자료구조

- [x] 1.1 `apps/api/internal/bot/harvester_consumer.go`에 `nodeStats` 구조체(5개 `atomic.Uint64` 필드: `pinsCreated`, `deduped`, `skipped`, `failed`, `adapterFallback`)를 정의한다.
- [x] 1.2 `NodeStatsSnapshot` 평문 구조체(uint64 5개)와 `NodeStats() NodeStatsSnapshot` 메서드를 노출한다. doc string에 "스냅샷은 비원자적이다"를 명시한다.

## 2. processOne 카운터 증가

- [x] 2.1 `createPins`의 반환을 `([]uuid.UUID, bool, error)`로 변경하여 `HarvestPipeline.ProcessDocument`가 돌려준 `created bool`을 흘려보낸다.
- [x] 2.2 fetch 실패 경로에서 `failed++`를 호출한다(기존 `fetchFailureCount` 유지).
- [x] 2.3 `extractDocument` 실패 경로에서 `failed++`를 호출한다.
- [x] 2.4 `fellBack == true`이면 `adapterFallback++`를 호출한다(주 카테고리 증가와 독립).
- [x] 2.5 classifier가 `pinnable=false`이면 `skipped++`를 호출하고 그 외 경로는 진입하지 않는다.
- [x] 2.6 `createPins` 실패 경로에서 `failed++`를 호출한다.
- [x] 2.7 createPins 성공이고 `created=true`이면 `pinsCreated++`, 아니면 `deduped++`를 호출한다.

## 3. 테스트

- [x] 3.1 `TestHarvesterConsumer_NodeStats_PinsCreated`: 성공 + 신규 insert 시 PinsCreated만 1 증가하고 나머지 4개는 0임을 검증한다.
- [x] 3.2 `TestHarvesterConsumer_NodeStats_Deduped`: 성공 + 기존 update 시 Deduped만 1 증가함을 검증한다.
- [x] 3.3 `TestHarvesterConsumer_NodeStats_Skipped`: classifier가 pinnable=false를 반환할 때 Skipped만 1 증가함을 검증한다.
- [x] 3.4 `TestHarvesterConsumer_NodeStats_Failed`: fetch/parse/pin_create 각 실패 경로에서 Failed만 1 증가함을 검증한다(3개 sub-case).
- [x] 3.5 `TestHarvesterConsumer_NodeStats_AdapterFallback`: 어댑터가 에러를 반환하고 generic으로 fallback해 Pin 생성에 성공할 때, PinsCreated와 AdapterFallback이 모두 1 증가함을 검증한다.
- [x] 3.6 `TestHarvesterConsumer_NodeStats_MutualExclusion`: 한 노드 처리 후 PinsCreated + Deduped + Skipped + Failed = 1 임을 N=4 노드 시나리오로 검증한다.

## 4. 회귀 확인

- [x] 4.1 `go build ./...` 통과.
- [x] 4.2 `go test ./apps/api/internal/bot/...` 통과(기존 테스트 무회귀).
- [x] 4.3 `openspec validate fix-harvester-node-stats --strict` 통과.
