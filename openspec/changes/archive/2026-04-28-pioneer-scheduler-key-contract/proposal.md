## Why

`URLScheduler` 계약(scheduler-claim-api)은 `SetStatus(key, ...)`의 `key`를 "이전 `Dequeue`가 반환했거나 이전 `Enqueue` 호출에 전달된 URL 문자열" 로 정의하고 "내부적으로는 해당 URL의 정규화 결과로부터 유도된 `url_hash`로 lookup된다" 고 명시한다. 그러나 현재 구현(`PGURLScheduler.SetStatus`)은 정규화 단계를 생략하고 `hashKey(key) = sha256(key)`를 그대로 사용한다. 정규화가 URL 형태를 변경하는 입력(예: `www.` 제거, fragment/query 제거, trailing slash 변경)에 대해 hash가 일치하지 않아 SetStatus가 0 rows를 업데이트하고 row의 `last_fetched_at`/`next_fetch_at`이 갱신되지 않는다.

QA에서 `pioneer pixiv` 90초 실행 시 39회 fetch 중 14회(36%)가 `WARN scheduler.set_status_fetched: unknown key (row not in frontier)`를 발생시켰다. 시드 `https://www.pixiv.net`이 `normalized_url=https://pixiv.net`로 저장된 row는 영원히 fetched로 마크되지 않고 lease 만료(10분)마다 재-claim되어 무한 재크롤 루프에 진입한다.

## What Changes

- `URLScheduler` 계약: `SetStatus`/`RecordFetchError`/`RecordHarvestError`가 `key` 인자를 lookup 전에 **반드시 동일 정규화 단계를 거쳐** `url_hash`를 산출해야 함을 명시한다.
- 동일 계약을 `EnqueueHarvester(rawURL, snapshotKey)`에도 적용한다(현재 정규화 수행 중이지만 spec에 반영되지 않음).
- 정규화가 URL 형태를 변경하는 모든 입력에 대해 `Dequeue` 반환값이 그대로 `SetStatus`/`RecordFetchError`/`RecordHarvestError`에 전달 가능함을 보장한다.
- 기존 implementation 결함을 수정한다: hash 불일치로 인한 silent miss 0건이 되어야 한다.

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities

- `scheduler`: `SetStatus`/`RecordFetchError`/`RecordHarvestError`/`EnqueueHarvester`의 `key` 인자 lookup 시 정규화 일관성 요구사항 추가. Dequeue 반환값과 SetStatus 인자 간 round-trip invariant 추가.

## Impact

- **코드**:
  - `apps/api/internal/scheduler/postgres_scheduler.go`: `SetStatus`/`RecordFetchError`/`RecordHarvestError`의 key→hash 변환에 정규화 적용. 또는 동등한 호출자 측 강제.
  - `apps/api/internal/scheduler/url_scheduler.go`: `hashKey` 시그니처 또는 호출 사이트 정리.
  - `apps/api/internal/bot/pioneer_consumer.go`: Pioneer가 fetch 성공/실패 시 SetStatus에 넘기는 key가 계약을 만족하도록 보장. `processOne`에서 이미 계산된 `canonical`을 활용 가능.
  - `apps/api/internal/bot/harvester_consumer.go`: 동일 계약 검증.
- **테스트**:
  - 정규화가 URL 형태를 바꾸는 입력(`www.` 제거, fragment 제거 등)으로 round-trip 회귀 테스트 추가.
  - 기존 scheduler 통합 테스트는 그대로 통과해야 한다(behavior 변경 없음).
- **운영**:
  - 무한 재크롤 루프 종료. 기존 lease 만료된 row들은 다음 Pioneer 실행에서 정상 fetched 마크됨.
  - 메트릭/로그상 `unknown key (row not in frontier)` warning 0건이 정상 상태가 된다.
- **스코프 외**:
  - DomainFilter cross-domain 허용 이슈 — 별도 change.
  - bot_graph_nodes 미생성 이슈 — 별도 change.
  - Dequeue 반환값을 raw URL → normalized URL로 바꾸는 시그니처 변경은 본 change에서 다루지 않는다(호환성 영향 큼).
