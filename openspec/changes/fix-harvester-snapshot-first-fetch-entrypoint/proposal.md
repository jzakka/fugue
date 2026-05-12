## Why

`openspec/specs/harvester/spec.md` L563-657에서 정의한 5개 Requirement(consumer가 snapshot-first 진입점만 경유, `(ctx, url)` 입력, 3-tuple `(html, errorKind, err)` 반환, 4종 errorKind 한정, 스냅샷 내부 실패 은닉)가 코드에 미구현되어 있습니다. Consumer는 `apps/api/internal/bot/harvester_consumer.go:313`에서 저수준 `Fetcher.Fetch`를 직접 호출하고, `:321, :490-527`에서 에러 메시지를 regex로 다시 파싱해 `errorKind`를 재분류합니다. 이는 archive change `2026-04-23-harvester-snapshot-first-fetchconsumer`가 spec에 도입한 fetch 경계 계약을 위반하며, 향후 fetch 경로 변경 시 consumer까지 수정해야 하는 경계 누수를 만듭니다.

## What Changes

- Consumer가 호출하는 fetch 경계를 snapshot-first 진입점 한 곳으로 좁힙니다. `Fetcher.Fetch` 직접 호출과 consumer 측 errorKind 재분류 로직(`classifyHarvestFetchError`, `harvestStatusCodeFromErr`, `harvestHTTPStatusErrorPattern`)을 제거합니다.
- snapshot-first 진입점은 `(ctx, url)`을 입력 받아 `(html, errorKind, err)` 의미론으로 결과를 반환합니다. 실패 시 `errorKind`는 `http_4xx`/`http_5xx`/`network`/`timeout` 4종으로 한정됩니다.
- ObjectStorage 조회 실패(키 없음, 만료, 네트워크, 권한, 내부 에러)는 진입점 내부에서 HTTP fallback으로 흡수되어 consumer에 노출되지 않습니다. ctx 취소/deadline은 `timeout`으로 귀결됩니다.
- production wiring(`apps/api/cmd/bot/*`)은 새 진입점을 consumer에 주입하도록 변경됩니다.

## Capabilities

### New Capabilities
<!-- 본 change는 신규 capability를 도입하지 않습니다. -->

### Modified Capabilities
- `harvester`: archive change `2026-04-23-harvester-snapshot-first-fetchconsumer`가 도입한 5개 Requirement(`openspec/specs/harvester/spec.md` L563-657)를 본 change의 `specs/harvester/spec.md`에 `## MODIFIED Requirements`로 동일 텍스트 재진술합니다. 본 델타는 행위 계약의 신규/변경/제거를 도입하지 않는 traceability 전용 재진술이며, archive 단계에서 기존 spec 본문에 동일 텍스트로 병합되어 no-op이 됩니다. 본 change의 실질 작업은 코드 측 구현이며, 그 작업이 어떤 Requirement의 어떤 Scenario를 enforce하는지 명시하기 위한 목적에서만 spec 델타를 둡니다.

## Impact

- `apps/api/internal/bot/`: 새 진입점 파일 추가, `harvester_consumer.go` 의존 타입과 호출 사이트 교체, 재분류 함수 삭제.
- `apps/api/cmd/bot/`: production wiring에서 `CompositeFetcher`를 새 진입점에 주입.
- Scheduler 계약(`scheduler.Dequeue`/`SetStatus`/`RecordHarvestError`)은 변경되지 않습니다. consumer가 `RecordHarvestError`에 전달하는 fetch 실패 kind만 진입점 반환값을 그대로 사용하는 형태로 바뀝니다.
- 기존 단위 테스트 중 `classifyHarvestFetchError`를 직접 검증하던 테스트는 새 진입점 단위 테스트로 대체됩니다.
