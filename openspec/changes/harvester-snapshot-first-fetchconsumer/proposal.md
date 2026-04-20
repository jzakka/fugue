## Why

`harvester-scheduler-consumer`의 consumer 루프는 fetch 단계를 `fetchSnapshotOrLive(ctx, url)`로 호출한다고 의사코드에 등장하지만, 이 진입점의 반환 형태와 호출 규약을 행위 계약으로 고정하지 않는다. 반면 `harvester-snapshot-first-fetch`는 `Fetcher.Fetch(url) ([]byte, error)` 시그니처만 명세하므로, consumer가 필요한 "fetch 단 실패 종류(`errorKind`)"를 자체 추론해야 한다. 두 spec 사이에 "루프 ↔ fetcher" 어댑터의 반환 형태·errorKind 범위·의존성 방향이 비어 있어, 구현이 만났을 때 (1) consumer에 HTTP 상태코드/에러 타입 분기 로직이 유출되거나, (2) `RecordHarvestError`에 fetch 단에서 나올 수 없는 kind가 섞이는 등의 표류가 발생할 수 있다.

본 change는 이 빈 칸만 정확히 메운다: consumer가 snapshot-first fetch를 어떻게 호출하고, 무엇을 받아 `RecordHarvestError`에 전달하는지를 행위 계약으로 확정한다. 새 기능은 도입하지 않으며, 두 기존 change의 경계 조건만 봉합한다.

## What Changes

- `harvester` capability에 "snapshot-first fetch consumer 진입 규약" requirement 묶음을 **추가**한다. 구체적으로:
  - Consumer는 fetch 단계에서 snapshot-first 진입점만 호출하고, ObjectStorage/HTTP 클라이언트를 직접 의존하지 않는다.
  - 진입점은 `(ctx, url)` 두 입력을 받고 `(html, errorKind, err)` 세 결과를 반환한다. 즉 consumer가 받는 반환 형태에 `errorKind`가 포함되어야 한다(기존 `Fetcher.Fetch`의 `([]byte, error)` 형태를 직접 호출하지 않는다).
  - 실패 반환 시 `errorKind`는 fetch 단 4종(`"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"`) 중 하나여야 한다. `"parse"`, `"pin_create"`는 fetch 이후 단계의 책임이며 본 진입점에서 반환되지 않는다.
  - Consumer는 진입점이 반환한 `errorKind`를 **fetch 실패 경로에 한해** 그대로 `scheduler.RecordHarvestError(url, errorKind)`에 전달하고, HTTP 상태코드/에러 타입을 다시 검사해 kind를 재결정하지 않는다. `SetStatus`/`RecordHarvestError` 이중 호출 규약 자체는 `harvester-scheduler-consumer`가 정의하며 본 change는 그 규약을 변경하지 않는다.
  - ObjectStorage 내부 실패 종류(키 없음/만료/네트워크/권한/내부)는 `harvester-snapshot-first-fetch`의 "단일 miss" 규약에 따라 진입점 내부에서 흡수되며, consumer 관점의 반환 `errorKind`에 반영되지 않는다. 본 change는 이 사실을 consumer-fetcher 경계의 관찰 계약으로 재확인한다(행위 신규 추가는 아님).
- 스냅샷 키 계산 방식(fetcher 내부에서 `pioneer-snapshot-storage` 공용 빌더로 URL에서 계산)은 `harvester-snapshot-first-fetch` Decision 5를 그대로 따른다. 본 change는 이 결정을 override하지 않는다.
- 범위 외(별도 change에서 다룸):
  - `CompositeFetcher`/`ObjectStorageFetcher`/`HTTPFetcher`의 내부 합성·실패 분류 규칙 → `harvester-snapshot-first-fetch`.
  - Consumer 루프의 전체 단계 순서, `SetStatus`/`RecordHarvestError` 이중 호출 규약, `errorKind` 전체 집합(`parse`/`pin_create` 포함) → `harvester-scheduler-consumer`.
  - HTTP fallback 결과의 ObjectStorage 재저장 정책 → `pioneer-snapshot-storage`.
  - Fetch retry/backoff 수식 → `scheduler-retry-backoff`.
  - `scheduler.Dequeue(queueType QueueType) (url string, err error)` 시그니처 변경 → 본 change는 변경하지 않으며 기존 계약을 그대로 전제한다.

## Capabilities

### Modified Capabilities
- `harvester`: `harvester-scheduler-consumer`가 도입한 capability에 ADDED 5건("fetch 단 진입점 호출 규약" — 경유 제약, 입력 형태, 반환 3-tuple, 4종 errorKind, 스냅샷 내부 실패 은닉)만 추가. MODIFIED/REMOVED/RENAMED 없음.

## Impact

- **코드**: `apps/api/internal/bot/` 하위 consumer 루프와 `harvester-snapshot-first-fetch` 구현 모듈 사이의 어댑터. 구현 시 snapshot-first 진입점이 `Fetcher.Fetch`를 내부 호출해 `([]byte, error)`를 `(html, errorKind, err)` 4종 kind로 매핑한다.
- **의존**: `harvester-scheduler-consumer`(선결), `harvester-snapshot-first-fetch`(선결), `scheduler-claim-api`의 기존 `Dequeue(queueType) (url, err)` 계약(변경 없음).
- **운영**: 동작 변화 없음. consumer가 받는 `errorKind`가 4종으로 한정됨이 명시되어, 알람/대시보드에서 fetch 단 실패와 이후 단계(`parse`/`pin_create`) 실패를 선형적으로 구분할 근거가 생긴다.
- **테스트 전략(참고)**: 진입점 반환값의 4종 제약과 consumer 재분류 금지는 구현 단의 단위 테스트로 검증한다. 본 spec의 수용 기준은 production behavior만 규정한다.
- **범위 외**: 실제 `CompositeFetcher` 합성 동작 테스트는 `harvester-snapshot-first-fetch`가 책임진다.
